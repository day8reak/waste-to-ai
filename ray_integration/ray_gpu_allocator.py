#!/usr/bin/env python3
"""
Ray Integration with GPU Scheduler

This module demonstrates how to integrate Ray with GPU Scheduler
for dynamic GPU resource management.

Usage:
    # Start Ray head node
    ray start --head --num-gpus=0

    # Run this script to allocate GPUs via GPU Scheduler
    python ray_gpu_allocator.py --job-id my-ray-job --num-gpus 4

    # Then start Ray workers with allocated GPUs
    ray start --address=$HEAD_ADDRESS
"""

import os
import sys
import argparse
import subprocess
from typing import List, Optional

# Add parent directory to path
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from ray_integration.gpu_scheduler_client import GPUSchedulerClient


class RayGPUAllocator:
    """
    Allocates GPUs from GPU Scheduler for Ray cluster.
    """

    def __init__(self, job_id: str, scheduler_url: str = "http://localhost:8080"):
        self.job_id = job_id
        self.client = GPUSchedulerClient(scheduler_url)

    def allocate_for_ray(self, num_gpus: int, gpu_model: Optional[str] = None) -> List[str]:
        """
        Allocate GPUs for Ray worker.

        Args:
            num_gpus: Number of GPUs to allocate
            gpu_model: Specific GPU model (optional)

        Returns:
            List of allocated GPU IDs
        """
        print(f"Requesting {num_gpus} GPUs from GPU Scheduler...")

        gpu_ids = self.client.allocate(
            job_id=self.job_id,
            num_gpus=num_gpus,
            gpu_model=gpu_model
        )

        print(f"Allocated GPUs: {gpu_ids}")
        return gpu_ids

    def setup_ray_environment(self, gpu_ids: List[str]):
        """
        Set up environment variables for Ray to use specific GPUs.

        Args:
            gpu_ids: List of GPU IDs to use
        """
        # Set CUDA visible devices
        os.environ["CUDA_VISIBLE_DEVICES"] = ",".join(gpu_ids)

        # Also set for Ray
        os.environ["RAY_EXTERNAL_GPU_IDS"] = ",".join(gpu_ids)

        print(f"Environment set: CUDA_VISIBLE_DEVICES={','.join(gpu_ids)}")

    def release_gpus(self, gpu_ids: Optional[List[str]] = None):
        """
        Release GPUs back to GPU Scheduler.

        Args:
            gpu_ids: GPU IDs to release (None = release all)
        """
        result = self.client.release(self.job_id, gpu_ids)
        print(f"Released GPUs: {result}")

    def block_gpus(self, gpu_ids: List[str]):
        """
        Block GPUs (release for other use, but keep using).

        Args:
            gpu_ids: GPU IDs to block
        """
        result = self.client.block_gpus(gpu_ids)
        print(f"Blocked GPUs: {result}")

    def unblock_gpus(self, gpu_ids: List[str]):
        """
        Unblock GPUs (restore for inference use).

        Args:
            gpu_ids: GPU IDs to unblock
        """
        result = self.client.unblock_gpus(gpu_ids)
        print(f"Unblocked GPUs: {result}")


def start_ray_worker(gpu_ids: List[str], head_address: Optional[str] = None):
    """
    Start Ray worker with specific GPUs.

    Args:
        gpu_ids: GPUs to use
        head_address: Ray head node address
    """
    # Set CUDA visible devices
    cuda_devices = ",".join(gpu_ids)

    cmd = [
        "ray", "start",
        "--num-gpus", str(len(gpu_ids)),
        "--env", f"CUDA_VISIBLE_DEVICES={cuda_devices}"
    ]

    if head_address:
        cmd.extend(["--address", head_address])
    else:
        cmd.append("--head")

    print(f"Starting Ray worker: {' '.join(cmd)}")
    subprocess.run(cmd)


def main():
    parser = argparse.ArgumentParser(
        description="Ray GPU Allocator - Integrate Ray with GPU Scheduler"
    )
    parser.add_argument(
        "--job-id",
        type=str,
        default="ray-cluster",
        help="Ray job ID"
    )
    parser.add_argument(
        "--url",
        type=str,
        default="http://localhost:8080",
        help="GPU Scheduler URL"
    )
    parser.add_argument(
        "--num-gpus",
        type=int,
        default=2,
        help="Number of GPUs to allocate"
    )
    parser.add_argument(
        "--gpu-model",
        type=str,
        help="Specific GPU model"
    )
    parser.add_argument(
        "--head-address",
        type=str,
        help="Ray head node address"
    )
    parser.add_argument(
        "--command",
        choices=["allocate", "start-worker", "block", "unblock", "status"],
        default="allocate",
        help="Command to execute"
    )
    parser.add_argument(
        "--gpus",
        nargs="+",
        help="GPU IDs for block/unblock commands"
    )

    args = parser.parse_args()

    # Create allocator
    allocator = RayGPUAllocator(args.job_id, args.url)

    if args.command == "allocate":
        # Allocate GPUs
        gpu_ids = allocator.allocate_for_ray(args.num_gpus, args.gpu_model)

        # Set environment
        allocator.setup_ray_environment(gpu_ids)

        print("\n" + "=" * 60)
        print("GPU allocation successful!")
        print(f"Job ID: {args.job_id}")
        print(f"GPUs: {gpu_ids}")
        print("\nTo start Ray worker:")
        print(f"  ray start --address=$HEAD_ADDRESS --num-gpus={len(gpu_ids)}")
        print(f"  export CUDA_VISIBLE_DEVICES={','.join(gpu_ids)}")
        print("=" * 60)

    elif args.command == "start-worker":
        # Allocate GPUs first
        gpu_ids = allocator.allocate_for_ray(args.num_gpus, args.gpu_model)

        # Start worker
        start_ray_worker(gpu_ids, args.head_address)

    elif args.command == "block":
        if not args.gpus:
            print("Error: --gpus required for block command")
            sys.exit(1)
        allocator.block_gpus(args.gpus)

    elif args.command == "unblock":
        if not args.gpus:
            print("Error: --gpus required for unblock command")
            sys.exit(1)
        allocator.unblock_gpus(args.gpus)

    elif args.command == "status":
        status = allocator.client.get_status()
        print(f"Total GPUs: {status['total_gpus']}")
        print(f"Available: {status['available_gpus']}")
        print(f"Allocated: {status['allocated_gpus']}")
        print(f"Blocked: {status.get('blocked_gpus', 0)}")


if __name__ == "__main__":
    main()
