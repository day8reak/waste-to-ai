#!/usr/bin/env python3
"""
vLLM Integration Example with GPU Scheduler

This example demonstrates how to integrate vLLM with GPU Scheduler
to enable dynamic GPU scaling.

Prerequisites:
    pip install vllm

Usage:
    python vllm_example.py --model facebook/opt-125m
"""

import argparse
import os
import signal
import subprocess
import sys
import time
from typing import List, Optional

# Add parent directory to path
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from inference_watcher import InferenceServiceManager


class VLLMManager:
    """
    Manages vLLM server with dynamic GPU scaling.
    """

    def __init__(
        self,
        job_id: str,
        model: str,
        scheduler_url: str = "http://localhost:8080",
        port: int = 8000
    ):
        self.job_id = job_id
        self.model = model
        self.scheduler_url = scheduler_url
        self.port = port
        self.process: Optional[subprocess.Popen] = None
        self.gpu_ids: List[str] = []

    def start_vllm(self, gpu_ids: List[str]):
        """
        Start vLLM server with specified GPUs.

        Args:
            gpu_ids: List of GPU IDs to use
        """
        # Stop existing vLLM if running
        if self.process:
            self.stop_vllm()

        if not gpu_ids:
            print("No GPUs available, skipping vLLM start")
            return

        self.gpu_ids = gpu_ids
        gpu_str = ",".join(gpu_ids)

        # Construct vLLM command
        cmd = [
            "python", "-m", "vllm.entrypoints.api_server",
            "--model", self.model,
            "--tensor-parallel-size", str(len(gpu_ids)),
            "--gpu-memory-utilization", "0.9",
            "--port", str(self.port)
        ]

        # Set CUDA visible devices
        env = os.environ.copy()
        env["CUDA_VISIBLE_DEVICES"] = gpu_str

        print(f"Starting vLLM with GPUs: {gpu_ids}")
        print(f"Command: {' '.join(cmd)}")

        self.process = subprocess.Popen(
            cmd,
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE
        )

        print(f"vLLM started (PID: {self.process.pid})")

    def stop_vllm(self):
        """Stop vLLM server."""
        if self.process:
            print(f"Stopping vLLM (PID: {self.process.pid})...")
            self.process.terminate()
            try:
                self.process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                self.process.kill()
            self.process = None
            print("vLLM stopped")

    def scale_vllm(self, gpu_ids: List[str]):
        """
        Scale vLLM to use new GPU set.

        Args:
            gpu_ids: New list of GPU IDs
        """
        if gpu_ids == self.gpu_ids:
            print(f"GPU set unchanged: {gpu_ids}")
            return

        print(f"Scaling vLLM from {self.gpu_ids} to {gpu_ids}")
        self.start_vllm(gpu_ids)


def main():
    parser = argparse.ArgumentParser(
        description="vLLM with GPU Scheduler Integration"
    )
    parser.add_argument(
        "--model",
        type=str,
        default="facebook/opt-125m",
        help="Model to serve"
    )
    parser.add_argument(
        "--job-id",
        type=str,
        default="vllm-inference",
        help="Job ID"
    )
    parser.add_argument(
        "--url",
        type=str,
        default="http://localhost:8080",
        help="GPU Scheduler URL"
    )
    parser.add_argument(
        "--port",
        type=int,
        default=8000,
        help="vLLM API port"
    )
    parser.add_argument(
        "--initial-gpus",
        type=int,
        default=2,
        help="Initial number of GPUs to allocate"
    )
    parser.add_argument(
        "--interval",
        type=int,
        default=5,
        help="Check interval in seconds"
    )

    args = parser.parse_args()

    # Create vLLM manager
    vllm_mgr = VLLMManager(
        job_id=args.job_id,
        model=args.model,
        scheduler_url=args.url,
        port=args.port
    )

    # Create inference service manager with callbacks
    manager = InferenceServiceManager(
        job_id=args.job_id,
        scheduler_url=args.url,
        check_interval=args.interval,
        on_scale_up=lambda gpus: vllm_mgr.scale_vllm(gpus),
        on_scale_down=lambda gpus: vllm_mgr.scale_vllm(gpus)
    )

    # Handle signals
    def signal_handler(sig, frame):
        print("\nShutting down...")
        manager.stop()
        vllm_mgr.stop_vllm()
        sys.exit(0)

    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)

    # Allocate initial GPUs
    print(f"Allocating {args.initial_gpus} GPUs...")
    gpu_ids = manager.allocate_gpus(args.initial_gpus)

    if gpu_ids:
        # Start vLLM with allocated GPUs
        vllm_mgr.start_vllm(gpu_ids)

    # Start watching for changes
    print("\nStarting GPU watcher...")
    print("Use 'python examples/ray_job_example.py block gpu1' to release a GPU")
    print("Use 'python examples/ray_job_example.py unblock gpu1' to restore a GPU")

    manager.watch()


if __name__ == "__main__":
    main()
