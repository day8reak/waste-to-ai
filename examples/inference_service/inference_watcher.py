#!/usr/bin/env python3
"""
GPU/NPU Scheduler Integration with Inference Service

This module provides integration between GPU Scheduler and inference services.
It monitors GPU availability changes and dynamically adjusts inference resources.

Usage:
    # As a standalone watcher
    python inference_watcher.py --job-id my-inference --mode watch

    # As a library
    from inference_watcher import InferenceServiceManager

    manager = InferenceServiceManager("my-inference", "http://localhost:8080")
    manager.start()
"""

import argparse
import os
import signal
import sys
import time
from typing import Dict, List, Optional, Set

# Add parent directory to path for imports
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from gpu_scheduler_client import GPUSchedulerClient


class InferenceServiceManager:
    """
    Manages inference service GPU resources with dynamic scaling.

    This class monitors GPU availability from the scheduler and
    dynamically adjusts the inference service resources.
    """

    def __init__(
        self,
        job_id: str,
        scheduler_url: str = "http://localhost:8080",
        check_interval: int = 5,
        on_scale_up=None,
        on_scale_down=None
    ):
        """
        Initialize the manager.

        Args:
            job_id: Ray/Task job ID
            scheduler_url: GPU Scheduler URL
            check_interval: Interval to check GPU availability (seconds)
            on_scale_up: Callback when GPUs are added
            on_scale_down: Callback when GPUs are removed
        """
        self.job_id = job_id
        self.client = GPUSchedulerClient(scheduler_url)
        self.check_interval = check_interval
        self.on_scale_up = on_scale_up
        self.on_scale_down = on_scale_down

        self.last_available_gpus: Set[str] = set()
        self.running = False

    def get_current_gpu_ids(self) -> Set[str]:
        """
        Get current GPU IDs from environment or inference service.

        In production, this should query the actual inference service
        (e.g., vLLM, Text Generation Inference).

        Returns:
            Set of currently used GPU IDs
        """
        # Method 1: From environment variable (set by Ray)
        cuda_visible = os.environ.get("CUDA_VISIBLE_DEVICES", "")
        if cuda_visible:
            return set(cuda_visible.split(","))

        # Method 2: Query scheduler for our job
        try:
            status = self.client.get_status()
            for task in status.get("ray_tasks", []):
                if task.get("ray_job_id") == self.job_id:
                    return set(task.get("gpu_assigned", []))
        except Exception as e:
            print(f"Error getting GPU status: {e}")

        return set()

    def get_available_gpu_ids(self) -> Set[str]:
        """
        Get available GPU IDs from scheduler (excludes blocked GPUs).

        Returns:
            Set of available GPU IDs
        """
        try:
            status = self.client.get_status()

            # Get all GPUs and filter by status
            gpus_response = self.client._request("GET", "/api/gpus")
            available = set()

            for gpu in gpus_response.get("gpus", []):
                if gpu.get("status") == "idle":
                    available.add(gpu.get("id"))

            return available
        except Exception as e:
            print(f"Error getting available GPUs: {e}")
            return set()

    def allocate_gpus(self, count: int, gpu_model: Optional[str] = None) -> List[str]:
        """
        Allocate GPUs from scheduler.

        Args:
            count: Number of GPUs to allocate
            gpu_model: Specific GPU model (optional)

        Returns:
            List of allocated GPU IDs
        """
        try:
            gpu_ids = self.client.allocate(
                job_id=self.job_id,
                num_gpus=count,
                gpu_model=gpu_model
            )
            print(f"Allocated GPUs: {gpu_ids}")
            return gpu_ids
        except Exception as e:
            print(f"Error allocating GPUs: {e}")
            return []

    def release_gpus(self, gpu_ids: List[str]):
        """
        Release specific GPUs.

        Args:
            gpu_ids: GPU IDs to release
        """
        try:
            result = self.client.release(self.job_id, gpu_ids)
            print(f"Released GPUs: {gpu_ids}, result: {result}")
        except Exception as e:
            print(f"Error releasing GPUs: {e}")

    def scale_to(self, target_count: int):
        """
        Scale inference service to use target number of GPUs.

        Args:
            target_count: Target number of GPUs
        """
        current = self.get_current_gpu_ids()
        current_count = len(current)

        if target_count == current_count:
            print(f"Already using {current_count} GPUs, no scaling needed")
            return

        if target_count > current_count:
            # Scale up
            additional = target_count - current_count
            print(f"Scaling up: adding {additional} GPUs")

            # Get available GPUs
            available = self.get_available_gpu_ids()

            # Find GPUs we can add (available + currently used)
            all_gpus = current | available
            new_gpus = list(all_gpus)[:target_count]

            if self.on_scale_up:
                self.on_scale_up(new_gpus)
            else:
                print(f"Would scale up to: {new_gpus}")

        else:
            # Scale down
            to_remove = current_count - target_count
            print(f"Scaling down: removing {to_remove} GPUs")

            current_list = list(current)
            gpus_to_release = current_list[:to_remove]

            if self.on_scale_down:
                self.on_scale_down(gpus_to_release)
            else:
                print(f"Would release: {gpus_to_release}")

    def watch(self):
        """
        Start watching for GPU availability changes.

        This method monitors the scheduler and calls the appropriate
        callbacks when GPU availability changes.
        """
        self.running = True

        print(f"Starting GPU watcher for job: {self.job_id}")
        print(f"Check interval: {self.check_interval} seconds")

        while self.running:
            try:
                # Get current available GPUs
                available = self.get_available_gpu_ids()

                # Detect changes
                added = available - self.last_available_gpus
                removed = self.last_available_gpus - available

                if added:
                    print(f"\n[+] New GPUs available: {added}")

                    # Auto-scale up if configured
                    if self.on_scale_up:
                        all_gpus = self.get_current_gpu_ids() | available
                        self.on_scale_up(list(all_gpus))

                if removed:
                    print(f"\n[-] GPUs no longer available: {removed}")

                    # Note: We don't auto-scale down because:
                    # 1. Blocked GPUs still work for current inference
                    # 2. The inference service continues running
                    # 3. Only new allocations are affected

                # Update last known state
                self.last_available_gpus = available

            except Exception as e:
                print(f"Error in watch loop: {e}")

            time.sleep(self.check_interval)

    def stop(self):
        """Stop the watcher."""
        self.running = False
        print("Watcher stopped")


def main():
    parser = argparse.ArgumentParser(
        description="GPU Scheduler Inference Service Manager"
    )
    parser.add_argument(
        "--job-id",
        type=str,
        default="inference-service",
        help="Job ID for the inference service"
    )
    parser.add_argument(
        "--url",
        type=str,
        default="http://localhost:8080",
        help="GPU Scheduler URL"
    )
    parser.add_argument(
        "--allocate",
        type=int,
        help="Number of GPUs to allocate on start"
    )
    parser.add_argument(
        "--mode",
        choices=["allocate", "watch", "both"],
        default="both",
        help="Mode: allocate GPUs, watch for changes, or both"
    )
    parser.add_argument(
        "--interval",
        type=int,
        default=5,
        help="Check interval in seconds"
    )
    parser.add_argument(
        "--gpu-model",
        type=str,
        help="Specific GPU model to request"
    )

    args = parser.parse_args()

    # Create manager
    manager = InferenceServiceManager(
        job_id=args.job_id,
        scheduler_url=args.url,
        check_interval=args.interval
    )

    # Handle signals
    def signal_handler(sig, frame):
        print("\nShutting down...")
        manager.stop()
        sys.exit(0)

    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)

    # Execute based on mode
    if args.mode in ["allocate", "both"]:
        if args.allocate:
            print(f"Allocating {args.allocate} GPUs...")
            gpus = manager.allocate_gpus(args.allocate, args.gpu_model)
            print(f"Allocated: {gpus}")

    if args.mode in ["watch", "both"]:
        manager.watch()


if __name__ == "__main__":
    main()
