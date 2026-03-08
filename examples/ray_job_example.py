#!/usr/bin/env python3
"""
Ray Job Example - GPU Scheduler Integration

This example demonstrates how to use the GPU Scheduler client
for Ray job GPU allocation and management.

Usage:
    # Allocate 4 GPUs for a Ray inference job
    python ray_job_example.py allocate llama-inference 4

    # Scale down to 2 GPUs
    python ray_job_example.py scale-down llama-inference 2

    # Release all GPUs
    python ray_job_example.py release llama-inference
"""

import sys
import time
from gpu_scheduler_client import GPUSchedulerClient


def main():
    if len(sys.argv) < 2:
        print_usage()
        sys.exit(1)

    command = sys.argv[1]
    base_url = "http://localhost:8080"
    client = GPUSchedulerClient(base_url)

    try:
        if command == "allocate":
            allocate_example(client)
        elif command == "scale-down":
            scale_down_example(client)
        elif command == "scale-up":
            scale_up_example(client)
        elif command == "release":
            release_example(client)
        elif command == "status":
            status_example(client)
        elif command == "block":
            block_example(client)
        elif command == "unblock":
            unblock_example(client)
        elif command == "demo":
            demo_workflow(client)
        else:
            print(f"Unknown command: {command}")
            print_usage()
            sys.exit(1)

    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)


def allocate_example(client: GPUSchedulerClient):
    """Example: Allocate GPUs for a Ray job."""
    if len(sys.argv) < 4:
        print("Usage: python ray_job_example.py allocate <job_id> <num_gpus> [gpu_model]")
        sys.exit(1)

    job_id = sys.argv[2]
    num_gpus = int(sys.argv[3])
    gpu_model = sys.argv[4] if len(sys.argv) > 4 else None

    print(f"Allocating {num_gpus} GPU(s) for job '{job_id}'...")
    if gpu_model:
        print(f"  GPU model: {gpu_model}")

    gpu_ids = client.allocate(job_id, num_gpus, gpu_model)

    print(f"Success! Allocated GPUs: {gpu_ids}")
    return gpu_ids


def scale_down_example(client: GPUSchedulerClient):
    """Example: Scale down a Ray job."""
    if len(sys.argv) < 4:
        print("Usage: python ray_job_example.py scale-down <job_id> <remaining_gpus>")
        sys.exit(1)

    job_id = sys.argv[2]
    remaining_gpus = int(sys.argv[3])

    print(f"Scaling down job '{job_id}' to {remaining_gpus} GPU(s)...")

    result = client.scale_down(job_id, remaining_gpus)

    print(f"Scale down result: {result}")
    return result


def scale_up_example(client: GPUSchedulerClient):
    """Example: Scale up a Ray job."""
    if len(sys.argv) < 4:
        print("Usage: python ray_job_example.py scale-up <job_id> <target_gpus>")
        sys.exit(1)

    job_id = sys.argv[2]
    target_gpus = int(sys.argv[3])

    print(f"Scaling up job '{job_id}' to {target_gpus} GPU(s)...")

    gpu_ids = client.scale_up(job_id, target_gpus)

    print(f"Scale up result: {gpu_ids}")
    return gpu_ids


def release_example(client: GPUSchedulerClient):
    """Example: Release GPUs for a Ray job."""
    if len(sys.argv) < 3:
        print("Usage: python ray_job_example.py release <job_id>")
        sys.exit(1)

    job_id = sys.argv[2]

    print(f"Releasing GPU(s) for job '{job_id}'...")

    result = client.release(job_id)

    print(f"Release result: {result}")
    return result


def status_example(client: GPUSchedulerClient):
    """Example: Get cluster status."""
    print("Getting cluster status...")

    status = client.get_status()

    print("=" * 50)
    print("Cluster Status")
    print("=" * 50)
    print(f"Total GPUs:      {status['total_gpus']}")
    print(f"Available GPUs:  {status['available_gpus']}")
    print(f"Allocated GPUs:  {status['allocated_gpus']}")
    print(f"Ray Tasks:       {status['total_ray_tasks']}")
    print("=" * 50)

    if status['ray_tasks']:
        print("\nRay Tasks:")
        for task in status['ray_tasks']:
            print(f"  - Job ID: {task.get('ray_job_id', 'N/A')}")
            print(f"    GPUs: {task.get('gpu_assigned', [])}")
            print(f"    Status: {task.get('status', 'N/A')}")
            print()

    return status


def block_example(client: GPUSchedulerClient):
    """Example: Block GPUs (CLI releases them for other use)."""
    if len(sys.argv) < 3:
        print("Usage: python ray_job_example.py block <gpu_ids...>")
        print("Example: python ray_job_example.py block gpu0 gpu1")
        sys.exit(1)

    gpu_ids = sys.argv[2:]

    print(f"Blocking GPU(s): {gpu_ids}...")
    print("  Blocked GPUs will NOT be used for inference until unblocked")

    result = client.block_gpus(gpu_ids)

    print(f"Block result: {result}")
    print("\nNote: Inference service should detect available GPUs and continue running")
    return result


def unblock_example(client: GPUSchedulerClient):
    """Example: Unblock GPUs (restore them for inference)."""
    if len(sys.argv) < 3:
        print("Usage: python ray_job_example.py unblock <gpu_ids...>")
        print("Example: python ray_job_example.py unblock gpu0")
        sys.exit(1)

    gpu_ids = sys.argv[2:]

    print(f"Unblocking GPU(s): {gpu_ids}...")
    print("  Unblocked GPUs will be available for inference again")
    print("  Inference service should detect and increase throughput")

    result = client.unblock_gpus(gpu_ids)

    print(f"Unblock result: {result}")
    return result


def demo_workflow(client: GPUSchedulerClient):
    """Demonstrate a complete workflow."""
    job_id = "llama-inference-demo"

    print("=" * 60)
    print("GPU Scheduler + Ray Integration Demo")
    print("=" * 60)

    # Step 1: Check initial status
    print("\n[1] Initial cluster status:")
    status = client.get_status()
    print(f"    Available GPUs: {status['available_gpus']}/{status['total_gpus']}")

    # Step 2: Allocate GPUs
    print(f"\n[2] Allocating 4 GPUs for job '{job_id}'...")
    gpu_ids = client.allocate(job_id, num_gpus=4)
    print(f"    Allocated: {gpu_ids}")

    # Step 3: Check status after allocation
    print("\n[3] Status after allocation:")
    status = client.get_status()
    print(f"    Available GPUs: {status['available_gpus']}/{status['total_gpus']}")
    print(f"    Ray Tasks: {status['total_ray_tasks']}")

    # Step 4: Simulate scale down (dynamic scaling)
    print("\n[4] Simulating scale down to 2 GPUs...")
    result = client.scale_down(job_id, remaining_gpus=2)
    print(f"    Result: {result}")

    # Step 5: Check status after scale down
    print("\n[5] Status after scale down:")
    status = client.get_status()
    print(f"    Available GPUs: {status['available_gpus']}/{status['total_gpus']}")

    # Step 6: Release all GPUs
    print(f"\n[6] Releasing all GPUs for job '{job_id}'...")
    result = client.release(job_id)
    print(f"    Result: {result}")

    # Step 7: Final status
    print("\n[7] Final cluster status:")
    status = client.get_status()
    print(f"    Available GPUs: {status['available_gpus']}/{status['total_gpus']}")

    print("\n" + "=" * 60)
    print("Demo completed successfully!")
    print("=" * 60)


def print_usage():
    """Print usage information."""
    print("Usage: python ray_job_example.py <command> [args]")
    print("")
    print("Commands:")
    print("  allocate <job_id> <num_gpus> [gpu_model]")
    print("      Allocate GPUs for a Ray job")
    print("")
    print("  scale-down <job_id> <remaining_gpus>")
    print("      Scale down a Ray job (release excess GPUs)")
    print("")
    print("  scale-up <job_id> <target_gpus>")
    print("      Scale up a Ray job (allocate more GPUs)")
    print("")
    print("  release <job_id>")
    print("      Release all GPUs for a Ray job")
    print("")
    print("  status")
    print("      Show cluster status")
    print("")
    print("  block <gpu_ids...>")
    print("      Block GPUs (CLI releases them, cannot be used for inference)")
    print("      Example: python ray_job_example.py block gpu0 gpu1")
    print("")
    print("  unblock <gpu_ids...>")
    print("      Unblock GPUs (restore for inference, increase throughput)")
    print("      Example: python ray_job_example.py unblock gpu0")
    print("")
    print("  demo")
    print("      Run a complete demo workflow")
    print("")
    print("Examples:")
    print("  python ray_job_example.py allocate llama-job 4")
    print("  python ray_job_example.py allocate llama-job 4 V100")
    print("  python ray_job_example.py scale-down llama-job 2")
    print("  python ray_job_example.py release llama-job")
    print("  python ray_job_example.py status")
    print("  python ray_job_example.py demo")


if __name__ == "__main__":
    main()
