#!/usr/bin/env python3
"""
GPU Scheduler 实机测试脚本 (Python版)
用法: python3 test_deployment.py [--url http://localhost:8080]
"""

import argparse
import json
import sys
import time
import urllib.request
import urllib.error
from typing import Dict, List, Any, Optional

# 颜色输出
GREEN = '\033[32m'
RED = '\033[31m'
YELLOW = '\033[33m'
BLUE = '\033[34m'
NC = '\033[0m'


class GPUSchedulerTester:
    def __init__(self, base_url: str):
        self.base_url = base_url.rstrip('/')
        self.results: List[Dict[str, Any]] = []

    def log(self, msg: str, color: str = ""):
        print(f"{color}{msg}{NC}")

    def log_success(self, msg: str):
        self.log(f"✓ {msg}", GREEN)

    def log_error(self, msg: str):
        self.log(f"✗ {msg}", RED)

    def log_info(self, msg: str):
        self.log(f"  {msg}", BLUE)

    def request(self, method: str, path: str, data: Optional[Dict] = None) -> Dict:
        url = f"{self.base_url}{path}"
        headers = {'Content-Type': 'application/json'}

        req_data = json.dumps(data).encode('utf-8') if data else None
        req = urllib.request.Request(url, data=req_data, headers=headers, method=method)

        try:
            with urllib.request.urlopen(req, timeout=30) as resp:
                body = resp.read().decode('utf-8')
                return {'status': resp.status, 'body': json.loads(body) if body else {}}
        except urllib.error.HTTPError as e:
            body = e.read().decode('utf-8')
            return {'status': e.code, 'body': json.loads(body) if body else {}, 'error': str(e)}
        except Exception as e:
            return {'status': 0, 'body': {}, 'error': str(e)}

    # ========== 测试用例 ==========

    def test_server_connectivity(self) -> bool:
        """测试服务器连接"""
        self.log_info("测试服务器连接...")
        result = self.request('GET', '/api/gpus')

        if result['status'] == 200 or result['error']:
            self.log_success("服务器可访问")
            return True

        self.log_error(f"连接失败: {result.get('error', 'Unknown')}")
        return False

    def test_gpu_list(self) -> bool:
        """测试GPU列表查询"""
        self.log_info("查询GPU列表...")
        result = self.request('GET', '/api/gpus')

        if result['status'] != 200:
            self.log_error(f"HTTP {result['status']}")
            return False

        gpus = result['body'].get('gpus', [])
        if not gpus:
            self.log_error("未检测到GPU")
            return False

        self.log_success(f"检测到 {len(gpus)} 个GPU")
        for gpu in gpus:
            print(f"  - {gpu['id']}: {gpu['model']} ({gpu['memory']}MB) [{gpu['status']}]")
        return True

    def test_task_submit(self) -> bool:
        """测试任务提交"""
        self.log_info("提交测试任务...")
        result = self.request('POST', '/api/tasks', {
            'name': 'deploy-test',
            'command': 'echo hello',
            'image': 'ubuntu:latest',
            'gpu_required': 1,
            'priority': 5
        })

        if result['status'] not in [200, 202]:
            self.log_error(f"HTTP {result['status']}: {result['body']}")
            return False

        task_id = result['body'].get('task_id')
        self.log_success(f"任务已提交: {task_id}")
        return True

    def test_task_list(self) -> bool:
        """测试任务列表查询"""
        self.log_info("查询任务列表...")
        result = self.request('GET', '/api/tasks')

        if result['status'] != 200:
            self.log_error(f"HTTP {result['status']}")
            return False

        tasks = result['body'].get('tasks', [])
        self.log_success(f"查询成功，共 {len(tasks)} 个任务")
        return True

    def test_ray_allocate(self) -> bool:
        """测试Ray GPU分配"""
        self.log_info("分配GPU...")
        result = self.request('POST', '/api/ray/allocate', {
            'job_id': 'test-ray-job',
            'gpu_count': 1,
            'priority': 10
        })

        if result['status'] not in [200, 202]:
            self.log_error(f"HTTP {result['status']}: {result['body']}")
            return False

        gpu_ids = result['body'].get('gpu_ids', [])
        self.log_success(f"已分配GPU: {gpu_ids}")
        return True

    def test_ray_release(self) -> bool:
        """测试Ray GPU释放"""
        self.log_info("释放GPU...")
        result = self.request('POST', '/api/ray/release', {
            'job_id': 'test-ray-job'
        })

        if result['status'] != 200:
            self.log_error(f"HTTP {result['status']}")
            return False

        self.log_success("GPU已释放")
        return True

    def test_ray_block(self) -> bool:
        """测试GPU阻塞"""
        # 先获取GPU列表
        result = self.request('GET', '/api/gpus')
        gpus = result['body'].get('gpus', [])

        if not gpus:
            self.log_error("无GPU可用")
            return False

        gpu_id = gpus[0]['id']
        self.log_info(f"阻塞GPU: {gpu_id}...")

        result = self.request('POST', '/api/ray/block', {
            'gpu_ids': [gpu_id]
        })

        if result['status'] != 200:
            self.log_error(f"HTTP {result['status']}")
            return False

        # 立即解除阻塞
        time.sleep(0.5)
        self.request('POST', '/api/ray/unblock', {'gpu_ids': [gpu_id]})

        self.log_success(f"GPU {gpu_id} 已阻塞后解除")
        return True

    def test_stats(self) -> bool:
        """测试统计信息"""
        self.log_info("查询统计信息...")
        result = self.request('GET', '/api/stats')

        if result['status'] != 200:
            self.log_error(f"HTTP {result['status']}")
            return False

        stats = result['body']
        self.log_success(f"pending={stats.get('pending', 0)}, "
                        f"running={stats.get('running', 0)}, "
                        f"completed={stats.get('completed', 0)}")
        return True

    def test_kill_task(self) -> bool:
        """测试任务终止"""
        # 提交任务
        self.log_info("提交可终止任务...")
        result = self.request('POST', '/api/tasks', {
            'name': 'kill-test',
            'command': 'sleep 60',
            'image': 'ubuntu:latest',
            'gpu_required': 1,
            'priority': 5
        })

        if result['status'] not in [200, 202]:
            self.log_error(f"提交失败: HTTP {result['status']}")
            return False

        task_id = result['body'].get('task_id')
        self.log_info(f"任务ID: {task_id}")

        # 等待启动
        time.sleep(1)

        # 终止任务
        self.log_info("终止任务...")
        result = self.request('DELETE', f'/api/tasks/{task_id}')

        if result['status'] != 200:
            self.log_error(f"HTTP {result['status']}")
            return False

        self.log_success("任务已终止")
        return True

    def test_concurrent_tasks(self) -> bool:
        """测试并发任务提交"""
        self.log_info("提交5个并发任务...")

        import threading

        def submit_task(n):
            self.request('POST', '/api/tasks', {
                'name': f'concurrent-{n}',
                'command': 'echo test',
                'image': 'ubuntu:latest',
                'gpu_required': 1,
                'priority': 5
            })

        threads = []
        for i in range(5):
            t = threading.Thread(target=submit_task, args=(i,))
            threads.append(t)
            t.start()

        for t in threads:
            t.join()

        self.log_success("并发任务提交完成")
        return True

    def test_gpu_blocking_workflow(self) -> bool:
        """测试GPU阻塞工作流"""
        # 获取GPU
        result = self.request('GET', '/api/gpus')
        gpus = result['body'].get('gpus', [])

        if len(gpus) < 2:
            self.log_error("需要至少2个GPU")
            return False

        gpu_id = gpus[0]['id']
        self.log_info(f"测试GPU: {gpu_id}")

        # 1. 阻塞GPU
        self.request('POST', '/api/ray/block', {'gpu_ids': [gpu_id]})
        time.sleep(0.5)

        # 2. 尝试分配 (应该跳过被阻塞的GPU)
        result = self.request('POST', '/api/ray/allocate', {
            'job_id': 'block-test',
            'gpu_count': 1
        })

        # 3. 解除阻塞
        self.request('POST', '/api/ray/unblock', {'gpu_ids': [gpu_id]})

        if result['status'] in [200, 202]:
            self.log_success("GPU阻塞工作流正常")
            return True

        self.log_error("GPU阻塞工作流异常")
        return False

    def test_task_lifecycle(self) -> bool:
        """测试任务完整生命周期"""
        self.log_info("测试任务生命周期...")

        # 1. 提交任务
        result = self.request('POST', '/api/tasks', {
            'name': 'lifecycle-test',
            'command': 'echo lifecycle',
            'image': 'ubuntu:latest',
            'gpu_required': 1,
            'priority': 5
        })

        if result['status'] not in [200, 202]:
            self.log_error("提交失败")
            return False

        task_id = result['body'].get('task_id')
        self.log_info(f"任务ID: {task_id}")

        # 2. 等待执行
        time.sleep(1)

        # 3. 查询状态
        result = self.request('GET', f'/api/tasks/{task_id}')

        # 4. 终止
        self.request('DELETE', f'/api/tasks/{task_id}')

        self.log_success("生命周期测试完成")
        return True

    def test_multi_gpu_allocation(self) -> bool:
        """测试多GPU分配"""
        self.log_info("测试多GPU分配...")

        result = self.request('GET', '/api/gpus')
        gpu_count = len(result['body'].get('gpus', []))

        if gpu_count < 2:
            self.log_info(f"只有{gpu_count}个GPU，跳过")
            return True

        # 尝试分配2个GPU
        result = self.request('POST', '/api/ray/allocate', {
            'job_id': 'multi-gpu-test',
            'gpu_count': min(2, gpu_count)
        })

        if result['status'] in [200, 202]:
            self.log_success(f"多GPU分配成功")
            return True

        self.log_error("多GPU分配失败")
        return False

    # ========== 运行测试 ==========

    def run_all_tests(self) -> bool:
        """运行所有测试"""
        self.log(f"\n{'='*50}", YELLOW)
        self.log("  开始实机测试", YELLOW)
        self.log(f"{'='*50}\n", YELLOW)

        tests = [
            ("服务器连接", self.test_server_connectivity),
            ("GPU列表查询", self.test_gpu_list),
            ("任务提交", self.test_task_submit),
            ("任务列表", self.test_task_list),
            ("Ray GPU分配", self.test_ray_allocate),
            ("Ray GPU释放", self.test_ray_release),
            ("GPU阻塞", self.test_ray_block),
            ("统计信息", self.test_stats),
            ("任务终止", self.test_kill_task),
            ("并发任务", self.test_concurrent_tasks),
            ("GPU阻塞工作流", self.test_gpu_blocking_workflow),
            ("任务生命周期", self.test_task_lifecycle),
            ("多GPU分配", self.test_multi_gpu_allocation),
        ]

        passed = 0
        failed = 0

        for name, test_func in tests:
            self.log(f"\n{'─'*40}", YELLOW)
            self.log(f"测试: {name}", YELLOW)
            self.log(f"{'─'*40}", YELLOW)

            try:
                if test_func():
                    passed += 1
                    self.results.append({'name': name, 'passed': True})
                else:
                    failed += 1
                    self.results.append({'name': name, 'passed': False})
            except Exception as e:
                self.log_error(f"异常: {e}")
                failed += 1
                self.results.append({'name': name, 'passed': False, 'error': str(e)})

        # 输出总结
        self.log(f"\n{'='*50}", YELLOW)
        self.log(f"  测试结果总结", YELLOW)
        self.log(f"{'='*50}\n", YELLOW)

        self.log(f"通过: {passed}", GREEN)
        self.log(f"失败: {failed}", RED if failed > 0 else GREEN)

        return failed == 0


def main():
    parser = argparse.ArgumentParser(description='GPU Scheduler 实机测试')
    parser.add_argument('--url', default='http://localhost:8080',
                       help='GPU Scheduler 服务器地址')
    parser.add_argument('--test', choices=['all', 'gpu', 'task', 'ray', 'stats'],
                       default='all', help='测试类型')
    parser.add_argument('--json', action='store_true', help='JSON输出')

    args = parser.parse_args()

    tester = GPUSchedulerTester(args.url)

    if args.test == 'all':
        success = tester.run_all_tests()
        sys.exit(0 if success else 1)
    elif args.test == 'gpu':
        tester.test_server_connectivity()
        tester.test_gpu_list()
    elif args.test == 'task':
        tester.test_task_submit()
        tester.test_task_list()
    elif args.test == 'ray':
        tester.test_ray_allocate()
        tester.test_ray_release()
        tester.test_ray_block()
    elif args.test == 'stats':
        tester.test_stats()


if __name__ == '__main__':
    main()
