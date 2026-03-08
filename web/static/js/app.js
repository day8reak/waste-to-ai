// GPU Scheduler Admin - Vue 3 Application

const { createApp, ref, computed, onMounted } = Vue;

// ============================================================================
// Backdoor: 输入 ↑↑↓↓←→←→BA 进入开发者模式
// ============================================================================

const BackdoorHandler = {
    sequence: [],
    secretCode: ['ArrowUp','ArrowUp','ArrowDown','ArrowDown','ArrowLeft','ArrowRight','ArrowLeft','ArrowRight','b','a'],
    enabled: false,
    clickCount: 0,
    lastClickTime: 0,

    check(key) {
        this.sequence.push(key);
        this.sequence = this.sequence.slice(-this.secretCode.length);

        if (this.sequence.join('').toLowerCase() === this.secretCode.join('').toLowerCase()) {
            this.enabled = true;
            this.sequence = [];
            return true;
        }
        return false;
    },

    // 连续点击 logo 5 次进入
    checkLogoClick() {
        const now = Date.now();
        if (now - this.lastClickTime < 500) {
            this.clickCount++;
        } else {
            this.clickCount = 1;
        }
        this.lastClickTime = now;

        if (this.clickCount >= 5) {
            this.enabled = true;
            this.clickCount = 0;
            return true;
        }
        return false;
    }
};

// ============================================================================
// GPU 数量生成器 - 修复状态分配逻辑
// ============================================================================

function generateGPUs(count) {
    const models = ['V100', 'A100', 'RTX 3090'];
    const gpus = [];

    for (let i = 0; i < count; i++) {
        const model = models[i % models.length];
        // 默认全部空闲
        gpus.push({
            id: `gpu${i}`,
            model: model,
            status: 'idle',
            task_id: null,
            task_name: null
        });
    }
    return gpus;
}

// ============================================================================
// 默认数据 (非 Mock 模式)
// ============================================================================

const DEFAULT_DATA = {
    gpus: generateGPUs(4),
    tasks: [],
    stats: { pending: 0, running: 0, completed: 0 },
    rayStatus: {
        total_gpus: 4,
        available_gpus: 4,
        allocated_gpus: 0,
        blocked_gpus: 0,
        ray_tasks: []
    }
};

// ============================================================================
// Mock Data - 修复版本
// ============================================================================

function createMockData(gpuCount = 4) {
    // 所有 GPU 默认都是空闲状态，没有任务
    const gpus = [];
    const models = ['V100', 'A100', 'RTX 3090'];
    for (let i = 0; i < gpuCount; i++) {
        gpus.push({
            id: `gpu${i}`,
            model: models[i % models.length],
            status: 'idle',
            task_id: null,
            task_name: null
        });
    }

    // 没有预置任务
    const tasks = [];

    return {
        gpus: gpus,
        tasks: tasks,
        stats: { pending: 0, running: 0, completed: 0 },
        rayStatus: {
            total_gpus: gpuCount,
            available_gpus: gpuCount,
            allocated_gpus: 0,
            blocked_gpus: 0,
            ray_tasks: []
        }
    };
}

// ============================================================================
// Mock API
// ============================================================================

class MockAPI {
    constructor() {
        this.useMock = false;
        this.data = createMockData(4);
    }

    reset(gpuCount = 4) {
        this.data = createMockData(gpuCount);
    }

    setMockMode(enabled, gpuCount = 4) {
        this.useMock = enabled;
        if (enabled) {
            this.reset(gpuCount);
        }
    }

    async fetchGPUs() {
        if (this.useMock) {
            return { gpus: this.data.gpus, total: this.data.gpus.length };
        }
        const res = await fetch('/api/gpus');
        return res.json();
    }

    async fetchTasks(status = '') {
        if (this.useMock) {
            let tasks = [...this.data.tasks];
            if (status) tasks = tasks.filter(t => t.status === status);
            return { tasks, total: tasks.length };
        }
        const url = status ? `/api/tasks?status=${status}` : '/api/tasks';
        const res = await fetch(url);
        return res.json();
    }

    async fetchStats() {
        if (this.useMock) {
            const stats = {
                pending: this.data.tasks.filter(t => t.status === 'pending').length,
                running: this.data.tasks.filter(t => t.status === 'running').length,
                completed: this.data.tasks.filter(t => t.status === 'completed').length
            };
            return stats;
        }
        const res = await fetch('/api/stats');
        return res.json();
    }

    async fetchRayStatus() {
        if (this.useMock) {
            this.syncRayStatus();
            return { ...this.data.rayStatus };
        }
        const res = await fetch('/api/ray/status');
        return res.json();
    }

    syncRayStatus() {
        let available = 0, allocated = 0;
        for (const gpu of this.data.gpus) {
            if (gpu.status === 'idle') available++;
            else if (gpu.status === 'allocated') allocated++;
        }
        this.data.rayStatus.total_gpus = this.data.gpus.length;
        this.data.rayStatus.available_gpus = available;
        this.data.rayStatus.allocated_gpus = allocated;
    }

    async submitTask(taskData) {
        if (this.useMock) {
            if (!taskData.command || !taskData.image) {
                throw new Error('command and image are required');
            }
            const gpuRequired = taskData.gpu_required > 0 ? taskData.gpu_required : 1;
            const minGpuRequired = taskData.min_gpu_required > 0 ? taskData.min_gpu_required : gpuRequired;
            const maxGpuRequired = taskData.max_gpu_required > 0 ? taskData.max_gpu_required : gpuRequired;
            const priority = taskData.priority > 0 ? taskData.priority : 5;
            const dynamic = taskData.dynamic === true;
            const assignedGPUs = this.allocateGPUsForTask(gpuRequired, taskData.gpu_model);

            const newTask = {
                id: 'task-' + Date.now(),
                name: taskData.name || '',
                command: taskData.command,
                image: taskData.image,
                gpu_required: gpuRequired,
                min_gpu_required: minGpuRequired,
                max_gpu_required: maxGpuRequired,
                dynamic: dynamic,
                gpu_model: taskData.gpu_model || '',
                priority: priority,
                gpu_assigned: assignedGPUs,
                status: assignedGPUs.length >= gpuRequired ? 'running' : 'pending',
                created_at: new Date().toISOString()
            };

            this.data.tasks.unshift(newTask);

            if (newTask.status === 'running') {
                assignedGPUs.forEach(gpuId => {
                    const gpu = this.data.gpus.find(g => g.id === gpuId);
                    if (gpu) { gpu.status = 'allocated'; gpu.task_id = newTask.id; }
                });
            }
            this.syncRayStatus();
            return { task_id: newTask.id, status: newTask.status, gpu_ids: assignedGPUs };
        }
        const res = await fetch('/api/tasks', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(taskData) });
        return res.json();
    }

    allocateGPUsForTask(gpuCount, gpuModel) {
        // 只分配idle状态的GPU
        let availableGPUs = this.data.gpus.filter(g => g.status === 'idle');
        if (gpuModel) availableGPUs = availableGPUs.filter(g => g.model === gpuModel);
        return availableGPUs.slice(0, gpuCount).map(g => g.id);
    }

    async killTask(taskId) {
        if (this.useMock) {
            const task = this.data.tasks.find(t => t.id === taskId);
            if (!task) throw new Error('task not found');
            if (task.status !== 'running') throw new Error('task is not running');

            if (task.gpu_assigned) {
                task.gpu_assigned.forEach(gpuId => {
                    const gpu = this.data.gpus.find(g => g.id === gpuId);
                    if (gpu) { gpu.status = 'idle'; gpu.task_id = null; }
                });
            }
            task.status = 'killed';
            this.syncRayStatus();
            return { message: 'task killed' };
        }
        const res = await fetch(`/api/tasks/${taskId}/kill`, { method: 'POST' });
        return res.json();
    }

    async blockGPU(gpuId) {
        if (this.useMock) {
            const gpu = this.data.gpus.find(g => g.id === gpuId);
            if (!gpu) throw new Error('GPU not found');
            // 释放GPU：从推理服务中真正释放该GPU
            // GPU变为idle，可给其他训练任务使用
            // 推理服务检测到GPU减少后会自动调整

            // 查找使用该GPU的任务并检查最低GPU保障
            const task = this.data.tasks.find(t => t.gpu_assigned && t.gpu_assigned.includes(gpuId));
            const rayTask = this.data.rayStatus.ray_tasks.find(t => t.gpu_assigned && t.gpu_assigned.includes(gpuId));

            const currentTask = task || rayTask;
            // 检查最低GPU保障
            if (currentTask && currentTask.dynamic && currentTask.min_gpu_required) {
                const currentGPUs = (currentTask.gpu_assigned || []).length;
                const remainingAfterRelease = currentGPUs - 1;
                if (remainingAfterRelease < currentTask.min_gpu_required) {
                    throw new Error(`该任务最低需要 ${currentTask.min_gpu_required} 个 GPU，当前 ${currentGPUs} 个，已达最低保障，无法释放`);
                }
            }

            // 从任务中移除该GPU
            if (task) {
                task.gpu_assigned = task.gpu_assigned.filter(id => id !== gpuId);
            }
            if (rayTask) {
                rayTask.gpu_assigned = rayTask.gpu_assigned.filter(id => id !== gpuId);
            }

            // 释放GPU
            gpu.status = 'idle';
            gpu.task_id = null;
            gpu.task_name = null;

            this.syncRayStatus();
            return { status: 'released', blocked: [gpuId], message: 'GPU released successfully. GPU is now idle and can be used by other tasks (e.g., training). Inference task will detect GPU loss and reduce throughput.' };
        }
        const res = await fetch('/api/ray/block', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ gpu_ids: [gpuId] }) });
        return res.json();
    }

    // unblock已废弃，保留接口兼容性
    async unblockGPU(gpuId) {
        if (this.useMock) {
            return { status: 'unblocked', unblocked: [gpuId], message: 'unblock is deprecated' };
        }
        const res = await fetch('/api/ray/unblock', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ gpu_ids: [gpuId] }) });
        return res.json();
    }

    // 强制释放指定 GPU（抢占）
    async releaseGPUById(gpuId) {
        if (this.useMock) {
            const gpu = this.data.gpus.find(g => g.id === gpuId);
            if (!gpu) throw new Error('GPU not found');

            // 查找使用该 GPU 的任务
            const task = this.data.tasks.find(t => t.gpu_assigned && t.gpu_assigned.includes(gpuId));
            const rayTask = this.data.rayStatus.ray_tasks.find(t => t.gpu_assigned && t.gpu_assigned.includes(gpuId));

            // 检查是否有最低 GPU 保护
            const currentTask = task || rayTask;
            if (currentTask && currentTask.min_gpu_required) {
                const minGpu = currentTask.min_gpu_required;
                const currentGpus = currentTask.gpu_assigned || [];
                const protectedCount = Math.min(currentGpus.length, minGpu);
                const gpuIndex = currentGpus.indexOf(gpuId);

                // 如果 GPU 在保护范围内，不能释放
                if (gpuIndex < protectedCount) {
                    throw new Error(`该任务最低需要 ${minGpu} 个 GPU，当前 ${currentGpus.length} 个，保护期内无法释放`);
                }
            }

            // 终止任务
            if (task) {
                task.status = 'killed';
                task.gpu_assigned = [];
            }

            // 查找 Ray 任务并释放
            if (rayTask) {
                rayTask.gpu_assigned = rayTask.gpu_assigned.filter(id => id !== gpuId);
                if (rayTask.gpu_assigned.length === 0) {
                    this.data.rayStatus.ray_tasks = this.data.rayStatus.ray_tasks.filter(t => t.id !== rayTask.id);
                }
            }

            // 释放 GPU
            gpu.status = 'idle';
            gpu.task_id = null;
            gpu.task_name = null;

            this.syncRayStatus();
            return { status: 'released', gpu_id: gpuId, message: `GPU ${gpuId} 已释放` };
        }
        // 真实 API 调用
        const res = await fetch('/api/ray/release', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ gpu_ids: [gpuId] }) });
        return res.json();
    }

    async allocateRayGPUs(allocData) {
        if (this.useMock) {
            if (!allocData.job_id) throw new Error('job_id is required');

            const gpuCount = allocData.gpu_count > 0 ? allocData.gpu_count : 1;
            // 只分配idle状态的GPU
            let availableGPUs = this.data.gpus.filter(g => g.status === 'idle');
            if (allocData.gpu_model) availableGPUs = availableGPUs.filter(g => g.model === allocData.gpu_model);

            // 如果用户指定了 GPU，优先使用指定的 GPU
            let assignedGPUs;
            if (allocData.selected_gpus && allocData.selected_gpus.length > 0) {
                // 验证选中的 GPU 是否可用
                const selectedAvailable = allocData.selected_gpus.filter(id => {
                    const gpu = this.data.gpus.find(g => g.id === id);
                    return gpu && gpu.status === 'idle';
                });
                if (selectedAvailable.length < gpuCount) {
                    throw new Error(`选中的 GPU 不足，需要 ${gpuCount} 个，可用的有 ${selectedAvailable.length} 个`);
                }
                assignedGPUs = selectedAvailable.slice(0, gpuCount);
            } else {
                assignedGPUs = availableGPUs.slice(0, gpuCount).map(g => g.id);
            }

            if (assignedGPUs.length === 0) throw new Error('no available GPUs');

            assignedGPUs.forEach(gpuId => {
                const gpu = this.data.gpus.find(g => g.id === gpuId);
                if (gpu) gpu.status = 'allocated';
            });

            const newTask = {
                id: 'ray-' + Date.now(),
                ray_job_id: allocData.job_id,
                gpu_required: gpuCount,
                min_gpu_required: allocData.min_gpu_required > 0 ? allocData.min_gpu_required : gpuCount,
                max_gpu_required: allocData.max_gpu_required > 0 ? allocData.max_gpu_required : gpuCount,
                dynamic: allocData.dynamic !== false, // 默认true
                gpu_model: allocData.gpu_model || '',
                priority: allocData.priority || 5,
                gpu_assigned: assignedGPUs,
                status: 'running',
                created_at: new Date().toISOString()
            };

            this.data.rayStatus.ray_tasks.push(newTask);
            this.syncRayStatus();
            return { task_id: newTask.id, job_id: allocData.job_id, status: 'running', gpu_ids: assignedGPUs, message: `allocated ${assignedGPUs.length} GPU(s) successfully` };
        }
        const res = await fetch('/api/ray/allocate', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(allocData) });
        return res.json();
    }

    async releaseGPUs(jobId, gpuIds = null) {
        if (this.useMock) {
            const task = this.data.rayStatus.ray_tasks.find(t => t.ray_job_id === jobId);
            if (!task) throw new Error('ray job not found');

            let gpusToRelease = gpuIds || [...(task.gpu_assigned || [])];

            // 检查最低 GPU 保护
            if (task.min_gpu_required) {
                const minGpu = task.min_gpu_required;
                const currentGpus = task.gpu_assigned || [];
                const protectedCount = Math.min(currentGpus.length, minGpu);

                // 保护从最后一个 GPU 开始的 min_gpu_required 个
                const protectedGPUs = currentGpus.slice(-protectedCount);
                const unprotectedGPUs = gpusToRelease.filter(id => !protectedGPUs.includes(id));

                if (unprotectedGPUs.length < gpusToRelease.length) {
                    // 有受保护的 GPU，提示用户
                    if (unprotectedGPUs.length === 0) {
                        throw new Error(`该任务最低需要 ${minGpu} 个 GPU，当前 ${currentGpus.length} 个，保护期内无法释放`);
                    }
                    // 只释放未受保护的 GPU
                    gpusToRelease = unprotectedGPUs;
                }
            }

            let releasedCount = 0;
            gpusToRelease.forEach(gpuId => {
                const gpu = this.data.gpus.find(g => g.id === gpuId);
                if (gpu && gpu.status === 'allocated') { gpu.status = 'idle'; gpu.task_id = null; gpu.task_name = null; releasedCount++; }
            });

            if (gpusToRelease.length >= (task.gpu_assigned || []).length) {
                this.data.rayStatus.ray_tasks = this.data.rayStatus.ray_tasks.filter(t => t.ray_job_id !== jobId);
            } else {
                task.gpu_assigned = task.gpu_assigned.filter(id => !gpusToRelease.includes(id));
                task.gpu_required = task.gpu_assigned.length;
            }

            this.syncRayStatus();
            return { job_id: jobId, status: 'released', message: `释放了 ${releasedCount} 个 GPU` };
        }
        const res = await fetch('/api/ray/release', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ job_id: jobId, gpu_ids: gpuIds }) });
        return res.json();
    }
}

const mockAPI = new MockAPI();

// ============================================================================
// Vue Application
// ============================================================================

const app = createApp({
    setup() {
        const currentView = ref('dashboard');
        const gpus = ref([]);
        const tasks = ref([]);
        const rayStatus = ref({ total_gpus: 0, available_gpus: 0, allocated_gpus: 0, blocked_gpus: 0, ray_tasks: [] });
        const stats = ref({ totalGPUs: 0, availableGPUs: 0, runningTasks: 0, pendingTasks: 0 });
        const isMockMode = ref(false);
        const showDevPanel = ref(false);
        const gpuCountOption = ref(4);
        const taskFilter = ref('');
        const newTask = ref({ name: '', command: '', image: '', gpu_required: 1, min_gpu_required: 1, max_gpu_required: 8, gpu_model: '', priority: 5, dynamic: false });
        const deployConfig = ref({ name: '', image: '', gpu_required: 1, gpu_model: '' });
        const rayAlloc = ref({ job_id: '', gpu_count: 1, min_gpu_required: 1, max_gpu_required: 8, gpu_model: '', priority: 5, selected_gpus: [], dynamic: true });
        const toast = ref({ show: false, message: '', type: 'success' });

        const presetModels = [
            { name: 'CodeGeeX', gpu: '1x V100', image: 'codegeex/codegeex:latest', icon: '💻', gpu_required: 1, gpu_model: 'V100' },
            { name: 'DeepSeek', gpu: '2x A100', image: 'deepseekai/deepseek-coder:latest', icon: '🧠', gpu_required: 2, gpu_model: 'A100' },
            { name: 'ChatGLM3', gpu: '1x V100', image: 'THUDM/chatglm3-streamline:latest', icon: '💬', gpu_required: 1, gpu_model: 'V100' },
            { name: 'Qwen', gpu: '2x A100', image: 'Qwen/Qwen-72B:latest', icon: '🌊', gpu_required: 2, gpu_model: 'A100' },
            { name: 'Llama2', gpu: '2x A100', image: 'meta-llama/Llama-2-70b:latest', icon: '🦙', gpu_required: 2, gpu_model: 'A100' },
            { name: 'Stable Diffusion', gpu: '1x V100', image: 'stabilityai/stable-diffusion:latest', icon: '🎨', gpu_required: 1, gpu_model: 'V100' }
        ];

        const pageTitle = computed(() => {
            const titles = { dashboard: '仪表盘', gpus: 'GPU 管理', tasks: '任务管理', deploy: '一键部署', ray: 'Ray 管理' };
            return titles[currentView.value] || '仪表盘';
        });

        const filteredTasks = computed(() => taskFilter.value ? tasks.value.filter(t => t.status === taskFilter.value) : tasks.value);
        const recentTasks = computed(() => tasks.value.slice(0, 5));

        const showToast = (message, type = 'success') => {
            toast.value = { show: true, message, type };
            setTimeout(() => toast.value.show = false, 3000);
        };

        const formatTime = (timestamp) => {
            if (!timestamp) return '-';
            const date = new Date(timestamp);
            return date.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' });
        };

        // 真实API调用函数
        const realAPI = {
            async fetchGPUs() {
                const res = await fetch('/api/gpus');
                return res.json();
            },
            async fetchTasks(status = '') {
                const url = status ? `/api/tasks?status=${status}` : '/api/tasks';
                const res = await fetch(url);
                return res.json();
            },
            async fetchStats() {
                const res = await fetch('/api/stats');
                return res.json();
            },
            async fetchRayStatus() {
                const res = await fetch('/api/ray/status');
                return res.json();
            },
            async blockGPU(gpuId) {
                const res = await fetch('/api/ray/block', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ gpu_ids: [gpuId] }) });
                return res.json();
            },
            async unblockGPU(gpuId) {
                const res = await fetch('/api/ray/unblock', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ gpu_ids: [gpuId] }) });
                return res.json();
            },
            async submitTask(task) {
                const res = await fetch('/api/tasks', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(task) });
                return res.json();
            },
            async killTask(taskId) {
                const res = await fetch(`/api/tasks/${taskId}/kill`, { method: 'POST' });
                return res.json();
            },
            async allocateRayGPUs(data) {
                const res = await fetch('/api/ray/allocate', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(data) });
                return res.json();
            },
            async releaseGPUs(jobId) {
                const res = await fetch('/api/ray/release', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ job_id: jobId }) });
                return res.json();
            }
        };

        // 根据模式选择API - 使用函数确保每次调用时都检查最新状态
        const getAPI = () => isMockMode.value ? mockAPI : realAPI;

        const fetchGPUs = async () => {
            try {
                const data = await getAPI().fetchGPUs();
                gpus.value = data.gpus || [];
                stats.value.totalGPUs = gpus.value.length;
                stats.value.availableGPUs = gpus.value.filter(g => g.status === 'idle').length;
            } catch (e) {
                console.error('Failed to fetch GPUs:', e);
            }
        };

        const fetchTasks = async () => {
            try {
                const data = await getAPI().fetchTasks();
                tasks.value = data.tasks || [];
                stats.value.runningTasks = tasks.value.filter(t => t.status === 'running').length;
                stats.value.pendingTasks = tasks.value.filter(t => t.status === 'pending').length;
            } catch (e) { console.error('Failed to fetch tasks:', e); }
        };

        const fetchStats = async () => {
            try {
                const data = await getAPI().fetchStats();
                stats.value.runningTasks = data.running || 0;
                stats.value.pendingTasks = data.pending || 0;
            } catch (e) { console.error('Failed to fetch stats:', e); }
        };

        const fetchRayStatus = async () => {
            try {
                const data = await getAPI().fetchRayStatus();
                rayStatus.value = data;
            } catch (e) { console.error('Failed to fetch Ray status:', e); }
        };

        const refreshData = async () => {
            await Promise.all([fetchGPUs(), fetchTasks(), fetchStats(), fetchRayStatus()]);
            showToast('数据已刷新', 'success');
        };

        const blockGPU = async (gpuId) => {
            try {
                await getAPI().blockGPU(gpuId);
                showToast(`GPU ${gpuId} 已释放，可用于其他任务`, 'success');
                await Promise.all([fetchGPUs(), fetchTasks(), fetchRayStatus()]);
            } catch (e) { showToast(e.message || '操作失败', 'error'); }
        };

        const unblockGPU = async (gpuId) => {
            try {
                await getAPI().unblockGPU(gpuId);
                showToast(`GPU ${gpuId} 已解除阻塞`, 'success');
                await Promise.all([fetchGPUs(), fetchTasks(), fetchRayStatus()]);
            } catch (e) { showToast(e.message || '操作失败', 'error'); }
        };

        // 强制释放 GPU（抢占）
        const releaseGPU = async (gpuId) => {
            if (!confirm(`确定要强制释放 GPU ${gpuId} 吗？这会终止使用该 GPU 的任务。`)) return;
            try {
                await mockAPI.releaseGPUById(gpuId);
                showToast(`GPU ${gpuId} 已释放`, 'success');
                await fetchGPUs();
                await fetchTasks();
                await fetchRayStatus();
            } catch (e) { showToast(e.message || '释放失败', 'error'); }
        };

        const submitTask = async () => {
            try {
                const data = await getAPI().submitTask(newTask.value);
                showToast(`任务已提交: ${data.task_id}`, 'success');
                newTask.value = { name: '', command: '', image: '', gpu_required: 1, min_gpu_required: 1, max_gpu_required: 8, gpu_model: '', priority: 5, dynamic: false };
                await fetchTasks();
                await fetchGPUs();
            } catch (e) { showToast(e.message || '提交失败', 'error'); }
        };

        const killTask = async (taskId) => {
            if (!confirm('确定要终止这个任务吗?')) return;
            try {
                await getAPI().killTask(taskId);
                showToast('任务已终止', 'success');
                await fetchTasks();
                await fetchGPUs();
            } catch (e) { showToast(e.message || '终止失败', 'error'); }
        };

        const deployModel = (model) => {
            deployConfig.value = { name: `${model.name} 部署`, image: model.image, gpu_required: model.gpu_required, gpu_model: model.gpu_model };
            currentView.value = 'deploy';
        };

        const submitDeployTask = async () => {
            if (!deployConfig.value.image) { showToast('请填写镜像地址', 'warning'); return; }
            const taskData = { name: deployConfig.value.name || '自定义部署', command: 'python -m http.server 8000', image: deployConfig.value.image, gpu_required: deployConfig.value.gpu_required, gpu_model: deployConfig.value.gpu_model, priority: 8 };
            try {
                await getAPI().submitTask(taskData);
                showToast('部署任务已提交', 'success');
                deployConfig.value = { name: '', image: '', gpu_required: 1, gpu_model: '' };
                await fetchTasks();
                await fetchGPUs();
            } catch (e) { showToast(e.message || '部署失败', 'error'); }
        };

        const allocateRayGPUs = async () => {
            if (!rayAlloc.value.job_id) { showToast('请填写 Job ID', 'warning'); return; }
            try {
                const data = await getAPI().allocateRayGPUs(rayAlloc.value);
                showToast(`已分配 ${data.gpu_ids?.length || 0} 个 GPU`, 'success');
                rayAlloc.value = { job_id: '', gpu_count: 1, min_gpu_required: 1, max_gpu_required: 8, gpu_model: '', priority: 5, selected_gpus: [], dynamic: true };
                await Promise.all([fetchGPUs(), fetchRayStatus()]);
            } catch (e) { showToast(e.message || '分配失败', 'error'); }
        };

        const releaseGPUs = async (jobId) => {
            if (!confirm(`确定要释放 Job ${jobId} 的 GPU 吗?`)) return;
            try {
                const data = await getAPI().releaseGPUs(jobId);
                showToast(data.message || 'GPU 已释放', 'success');
                await Promise.all([fetchGPUs(), fetchRayStatus()]);
            } catch (e) { showToast(e.message || '释放失败', 'error'); }
        };

        const toggleMockMode = () => {
            isMockMode.value = !isMockMode.value;
            mockAPI.setMockMode(isMockMode.value, gpuCountOption.value);
            showToast(isMockMode.value ? '🧪 Mock 模式已开启' : '🌐 真实 API 模式', 'success');
            refreshData();
        };

        const applyGPUCount = () => {
            mockAPI.reset(gpuCountOption.value);
            mockAPI.setMockMode(true, gpuCountOption.value);
            isMockMode.value = true;
            showToast(`已生成 ${gpuCountOption.value} 个 GPU`, 'success');
            refreshData();
        };

        const handleKeyPress = (e) => {
            if (BackdoorHandler.check(e.key)) {
                showDevPanel.value = true;
                isMockMode.value = true;
                mockAPI.setMockMode(true, gpuCountOption.value);
                showToast('🔓 开发者模式已解锁！', 'success');
                refreshData();
            }
        };

        const handleLogoClick = () => {
            if (BackdoorHandler.checkLogoClick()) {
                showDevPanel.value = true;
                isMockMode.value = true;
                mockAPI.setMockMode(true, gpuCountOption.value);
                showToast('🔓 开发者模式已解锁！', 'success');
                refreshData();
            }
        };

        onMounted(() => {
            document.addEventListener('keydown', handleKeyPress);
            refreshData();
        });

        return {
            currentView, gpus, tasks, rayStatus, stats, isMockMode, showDevPanel, gpuCountOption,
            taskFilter, newTask, deployConfig, rayAlloc, toast, presetModels,
            pageTitle, filteredTasks, recentTasks,
            formatTime, refreshData, blockGPU, unblockGPU, releaseGPU, submitTask, killTask,
            deployModel, submitDeployTask, allocateRayGPUs, releaseGPUs,
            toggleMockMode, applyGPUCount, handleLogoClick
        };
    }
});

app.mount('#app');
