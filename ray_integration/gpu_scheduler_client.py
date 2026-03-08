#!/usr/bin/env python3
"""
GPU Scheduler Python Client

A Python client library for interacting with the GPU Scheduler REST API.
Provides methods for allocating, releasing, and querying GPU resources.

Usage:
    from gpu_scheduler_client import GPUSchedulerClient

    client = GPUSchedulerClient("http://gpu-scheduler:8080")

    # Allocate GPUs
    gpu_ids = client.allocate("ray-job-123", num_gpus=4)

    # Get cluster status
    status = client.get_status()

    # Release GPUs
    client.release("ray-job-123")
"""

import requests
from typing import List, Optional, Dict, Any


class GPUSchedulerClient:
    """
    GPU Scheduler Python Client

    A client for interacting with the GPU Scheduler REST API.
    Supports GPU allocation, release, and status queries.
    """

    def __init__(
        self,
        base_url: str = "http://localhost:8080",
        timeout: int = 30
    ):
        """
        Initialize the GPU Scheduler client.

        Args:
            base_url: Base URL of the GPU Scheduler API
            timeout: Request timeout in seconds
        """
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout

    def _request(
        self,
        method: str,
        endpoint: str,
        json_data: Optional[Dict[str, Any]] = None
    ) -> Dict[str, Any]:
        """
        Make an HTTP request to the GPU Scheduler API.

        Args:
            method: HTTP method (GET, POST, etc.)
            endpoint: API endpoint path
            json_data: JSON request body

        Returns:
            Response JSON as dictionary

        Raises:
            requests.exceptions.RequestException: If the request fails
        """
        url = f"{self.base_url}{endpoint}"
        response = requests.request(
            method=method,
            url=url,
            json=json_data,
            timeout=self.timeout
        )
        response.raise_for_status()
        return response.json()

    def allocate(
        self,
        job_id: str,
        num_gpus: int = 1,
        gpu_model: Optional[str] = None,
        priority: int = 8
    ) -> List[str]:
        """
        Allocate GPUs for a Ray job.

        Args:
            job_id: Ray Job ID
            num_gpus: Number of GPUs to allocate
            gpu_model: Specific GPU model (e.g., "V100", "3090")
            priority: Job priority (1-10)

        Returns:
            List of allocated GPU IDs

        Raises:
            requests.exceptions.RequestException: If allocation fails
        """
        payload = {
            "job_id": job_id,
            "gpu_count": num_gpus,
            "priority": priority
        }

        if gpu_model:
            payload["gpu_model"] = gpu_model

        response = self._request("POST", "/api/ray/allocate", payload)

        if "error" in response:
            raise Exception(f"Allocation failed: {response['error']}")

        return response.get("gpu_ids", [])

    def release(
        self,
        job_id: str,
        gpu_ids: Optional[List[str]] = None
    ) -> Dict[str, Any]:
        """
        Release GPUs for a Ray job.

        Args:
            job_id: Ray Job ID
            gpu_ids: Specific GPU IDs to release (None = release all)

        Returns:
            Dictionary with release status and message

        Raises:
            requests.exceptions.RequestException: If release fails
        """
        payload = {
            "job_id": job_id
        }

        if gpu_ids:
            payload["gpu_ids"] = gpu_ids

        response = self._request("POST", "/api/ray/release", payload)

        if "error" in response:
            raise Exception(f"Release failed: {response['error']}")

        return {
            "status": response.get("status", "released"),
            "message": response.get("message", "")
        }

    def get_status(self) -> Dict[str, Any]:
        """
        Get the current cluster status.

        Returns:
            Dictionary containing:
                - total_gpus: Total number of GPUs
                - available_gpus: Number of available GPUs
                - allocated_gpus: Number of allocated GPUs
                - ray_tasks: List of Ray tasks
                - total_ray_tasks: Total number of Ray tasks

        Raises:
            requests.exceptions.RequestException: If request fails
        """
        response = self._request("GET", "/api/ray/status")
        return response

    def scale_down(
        self,
        job_id: str,
        remaining_gpus: int
    ) -> Dict[str, Any]:
        """
        Scale down a Ray job by releasing excess GPUs.

        Args:
            job_id: Ray Job ID
            remaining_gpus: Number of GPUs to keep

        Returns:
            Dictionary with scale down status

        Raises:
            requests.exceptions.RequestException: If scale down fails
        """
        # Get current allocation
        status = self.get_status()

        # Find the job
        job_task = None
        for task in status.get("ray_tasks", []):
            if task.get("ray_job_id") == job_id:
                job_task = task
                break

        if not job_task:
            raise Exception(f"Job {job_id} not found")

        current_gpus = job_task.get("gpu_assigned", [])
        if len(current_gpus) <= remaining_gpus:
            return {
                "status": "no_change",
                "message": f"Already using {len(current_gpus)} GPUs, no scaling needed"
            }

        # Calculate GPUs to release
        gpus_to_release = current_gpus[remaining_gpus:]

        return self.release(job_id, gpus_to_release)

    def scale_up(
        self,
        job_id: str,
        target_gpus: int
    ) -> List[str]:
        """
        Scale up a Ray job by allocating more GPUs.

        Note: This requires the job to already exist and be tracked.
        For new allocations, use allocate() directly.

        Args:
            job_id: Ray Job ID
            target_gpus: Target number of GPUs

        Returns:
            List of all GPU IDs after scaling

        Raises:
            requests.exceptions.RequestException: If scale up fails
        """
        status = self.get_status()

        # Find the job
        job_task = None
        for task in status.get("ray_tasks", []):
            if task.get("ray_job_id") == job_id:
                job_task = task
                break

        if not job_task:
            raise Exception(f"Job {job_id} not found")

        current_gpus = job_task.get("gpu_assigned", [])
        current_count = len(current_gpus)

        if current_count >= target_gpus:
            return current_gpus

        # Need to allocate more GPUs
        additional_gpus = target_gpus - current_count

        # Get current job info
        gpu_model = job_task.get("gpu_model", "")
        priority = job_task.get("priority", 8)

        # Note: This is a simplified approach
        # In production, you might want to release and re-allocate
        available = status.get("available_gpus", 0)

        if available < additional_gpus:
            raise Exception(
                f"Not enough available GPUs: need {additional_gpus}, "
                f"have {available}"
            )

        # Allocate additional GPUs (using a new job ID for simplicity)
        # In production, this should be handled differently
        new_gpu_ids = self.allocate(
            f"{job_id}-scaled",
            num_gpus=additional_gpus,
            gpu_model=gpu_model if gpu_model else None,
            priority=priority
        )

        return current_gpus + new_gpu_ids

    def block_gpus(self, gpu_ids: List[str]) -> Dict[str, Any]:
        """
        Block GPUs (CLI manually releases them for other use).

        Blocked GPUs will NOT be used for inference until unblocked.
        This is useful when you want to temporarily release GPUs
        for other purposes without stopping the inference service.

        Args:
            gpu_ids: List of GPU IDs to block

        Returns:
            Dictionary with block status and message

        Raises:
            requests.exceptions.RequestException: If block fails
        """
        payload = {"gpu_ids": gpu_ids}

        response = self._request("POST", "/api/ray/block", payload)

        if "error" in response:
            raise Exception(f"Block failed: {response['error']}")

        return {
            "status": response.get("status", "blocked"),
            "blocked": response.get("blocked", []),
            "message": response.get("message", "")
        }

    def unblock_gpus(self, gpu_ids: List[str]) -> Dict[str, Any]:
        """
        Unblock GPUs (restore them for inference use).

        Unblocked GPUs become available for inference again.
        The inference service should detect the newly available
        GPUs and increase throughput accordingly.

        Args:
            gpu_ids: List of GPU IDs to unblock

        Returns:
            Dictionary with unblock status and message

        Raises:
            requests.exceptions.RequestException: If unblock fails
        """
        payload = {"gpu_ids": gpu_ids}

        response = self._request("POST", "/api/ray/unblock", payload)

        if "error" in response:
            raise Exception(f"Unblock failed: {response['error']}")

        return {
            "status": response.get("status", "unblocked"),
            "unblocked": response.get("unblocked", []),
            "message": response.get("message", "")
        }

    def get_available_gpus(self) -> List[str]:
        """
        Get list of available GPUs (excludes blocked GPUs).

        This is the method inference services should call to
        determine which GPUs they can use.

        Returns:
            List of available GPU IDs
        """
        status = self.get_status()
        # Get all GPUs and filter
        gpus = self._request("GET", "/api/gpus")
        available = []
        for gpu in gpus.get("gpus", []):
            if gpu.get("status") == "idle":
                available.append(gpu.get("id"))
        return available


# Convenience functions

def allocate_gpus(
    job_id: str,
    num_gpus: int = 1,
    gpu_model: Optional[str] = None,
    base_url: str = "http://localhost:8080"
) -> List[str]:
    """
    Convenience function to allocate GPUs.

    Args:
        job_id: Ray Job ID
        num_gpus: Number of GPUs to allocate
        gpu_model: Specific GPU model
        base_url: GPU Scheduler base URL

    Returns:
        List of allocated GPU IDs
    """
    client = GPUSchedulerClient(base_url)
    return client.allocate(job_id, num_gpus, gpu_model)


def release_gpus(
    job_id: str,
    gpu_ids: Optional[List[str]] = None,
    base_url: str = "http://localhost:8080"
) -> Dict[str, Any]:
    """
    Convenience function to release GPUs.

    Args:
        job_id: Ray Job ID
        gpu_ids: Specific GPUs to release
        base_url: GPU Scheduler base URL

    Returns:
        Release status dictionary
    """
    client = GPUSchedulerClient(base_url)
    return client.release(job_id, gpu_ids)


def get_cluster_status(
    base_url: str = "http://localhost:8080"
) -> Dict[str, Any]:
    """
    Convenience function to get cluster status.

    Args:
        base_url: GPU Scheduler base URL

    Returns:
        Cluster status dictionary
    """
    client = GPUSchedulerClient(base_url)
    return client.get_status()


if __name__ == "__main__":
    import sys

    if len(sys.argv) < 2:
        print("Usage: python gpu_scheduler_client.py <command> [args]")
        print("")
        print("Commands:")
        print("  allocate <job_id> <num_gpus> [gpu_model]")
        print("  release <job_id> [gpu_ids...]")
        print("  status")
        sys.exit(1)

    command = sys.argv[1]
    client = GPUSchedulerClient()

    try:
        if command == "allocate":
            job_id = sys.argv[2]
            num_gpus = int(sys.argv[3]) if len(sys.argv) > 3 else 1
            gpu_model = sys.argv[4] if len(sys.argv) > 4 else None
            gpu_ids = client.allocate(job_id, num_gpus, gpu_model)
            print(f"Allocated GPUs: {gpu_ids}")

        elif command == "release":
            job_id = sys.argv[2]
            gpu_ids = sys.argv[3:] if len(sys.argv) > 3 else None
            result = client.release(job_id, gpu_ids)
            print(f"Release result: {result}")

        elif command == "status":
            status = client.get_status()
            print(f"Total GPUs: {status['total_gpus']}")
            print(f"Available: {status['available_gpus']}")
            print(f"Allocated: {status['allocated_gpus']}")
            print(f"Ray Tasks: {status['total_ray_tasks']}")

        else:
            print(f"Unknown command: {command}")
            sys.exit(1)

    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)
