const apiBase = 'http://127.0.0.1:3030';

function showToast(message, type = 'info') {
    const toast = document.createElement('div');
    toast.textContent = message;
    toast.style.cssText = `
        position: fixed;
        top: 20px;
        right: 20px;
        background: ${type === 'success' ? '#10b981' : type === 'error' ? '#ef4444' : '#3b82f6'};
        color: white;
        padding: 12px 16px;
        border-radius: 8px;
        font-size: 14px;
        z-index: 9999;
        box-shadow: 0 4px 12px rgba(0,0,0,0.3);
        animation: slideIn 0.3s ease-out;
    `;
    document.body.appendChild(toast);
    setTimeout(() => toast.remove(), 3000);
}

const style = document.createElement('style');
style.textContent = `
    @keyframes slideIn {
        from { transform: translateX(400px); opacity: 0; }
        to { transform: translateX(0); opacity: 1; }
    }
`;
document.head.appendChild(style);

async function startContainer(name) {
    try {
        console.log('Starting container:', name);
        const url = `${apiBase}/start/${encodeURIComponent(name)}`;
        console.log('Request URL:', url);
        const res = await fetch(url, { method: 'POST' });
        console.log('Response status:', res.status);
        if (res.ok) {
            showToast(`Started ${name}`, 'success');
            loadDashboard();
        } else {
            const msg = `Failed to start ${name} (${res.status})`;
            console.error(msg);
            showToast(msg, 'error');
        }
    } catch (e) {
        const msg = `Start error: ${e.message}`;
        console.error(msg, e);
        showToast(msg, 'error');
    }
}

async function stopContainer(name) {
    try {
        console.log('Stopping container:', name);
        const url = `${apiBase}/stop/${encodeURIComponent(name)}`;
        console.log('Request URL:', url);
        const res = await fetch(url, { method: 'POST' });
        console.log('Response status:', res.status);
        if (res.ok) {
            showToast(`Stopped ${name}`, 'success');
            loadDashboard();
        } else {
            const msg = `Failed to stop ${name} (${res.status})`;
            console.error(msg);
            showToast(msg, 'error');
        }
    } catch (e) {
        const msg = `Stop error: ${e.message}`;
        console.error(msg, e);
        showToast(msg, 'error');
    }
}

function fmtUptime(ts) {
    if (!ts) return '—';
    const now = Math.floor(Date.now() / 1000);
    const d = Math.max(0, now - ts);
    const days = Math.floor(d / 86400);
    const hours = Math.floor((d % 86400) / 3600);
    const minutes = Math.floor((d % 3600) / 60);
    if (days > 0) return `${days}d ${hours}h`;
    if (hours > 0) return `${hours}h ${minutes}m`;
    return `${minutes}m`;
}

function renderStats(containers) {
    const running = containers.filter(c => c.pid && c.pid > 0).length;
    document.getElementById('stat-running').innerText = running;
    document.getElementById('stat-running-delta').innerText = 'live';
    document.getElementById('stat-net').innerText = containers.find(c => c.isolated !== undefined)?.isolated ? 'isolated' : 'shared';
    document.getElementById('stat-images').innerText = containers.length || '—';
}

function renderTable(containers) {
    const tbody = document.getElementById('containers-body');
    if (!tbody) return;
    tbody.innerHTML = '';

    containers.forEach(c => {
        const tr = document.createElement('tr');

        const statusRunning = c.pid && c.pid > 0;
        const statusSpan = document.createElement('span');
        statusSpan.className = `status ${statusRunning ? 'running' : 'stopped'}`;
        statusSpan.innerHTML = `<span class="dot ${statusRunning ? 'running' : 'stopped'}"></span> ${statusRunning ? 'running' : 'stopped'}`;

        const cells = [
            c.name,
            statusSpan,
            c.image || '—',
            c.pid && c.pid > 0 ? c.pid : '—',
            '—',
            fmtUptime(c.timestamp),
            c.ports || 'none',
            c.isolated ? 'isolated' : 'shared',
            c.command && c.command.length ? c.command.join(' ') : '—'
        ];

        cells.forEach((val, idx) => {
            const td = document.createElement('td');
            if (val instanceof HTMLElement) td.appendChild(val); else td.textContent = val;
            if (idx === 1) td.style.whiteSpace = 'nowrap';
            tr.appendChild(td);
        });

        const actions = document.createElement('td');
        actions.className = 'quick-actions';
        const termBtn = document.createElement('button');
        termBtn.className = 'pill-button';
        termBtn.innerHTML = '<i class="fa-solid fa-terminal"></i>';
        termBtn.title = 'Open console';
        termBtn.onclick = (e) => { e.stopPropagation(); window.location.href = `/console.html?name=${encodeURIComponent(c.name)}`; };
        actions.appendChild(termBtn);
        
        const ctrlBtn = document.createElement('button');
        ctrlBtn.className = 'pill-button';
        if (statusRunning) {
            ctrlBtn.innerHTML = '<i class="fa-solid fa-stop"></i>';
            ctrlBtn.title = 'Stop container';
            ctrlBtn.onclick = (e) => { e.stopPropagation(); stopContainer(c.name); };
        } else {
            ctrlBtn.innerHTML = '<i class="fa-solid fa-play"></i>';
            ctrlBtn.title = 'Start container';
            ctrlBtn.onclick = (e) => { e.stopPropagation(); startContainer(c.name); };
        }
        actions.appendChild(ctrlBtn);
        tr.appendChild(actions);

        tbody.appendChild(tr);
    });
}

async function loadDashboard() {
    try {
        const res = await fetch(`${apiBase}/list`);
        const data = await res.json();
        const containers = data.containers || [];
        renderStats(containers);
        renderTable(containers);
    } catch (e) {
        console.error('dashboard load failed', e);
    }
}

function setupSidebar() {
    const items = document.querySelectorAll('.sidebar li');
    const sections = document.querySelectorAll('.section');
    items.forEach(li => {
        li.addEventListener('click', () => {
            const link = li.getAttribute('data-link');
            if (link) {
                window.open(link, '_blank');
                return;
            }
            const target = li.getAttribute('data-target');
            if (!target) return;
            items.forEach(i => i.classList.remove('active'));
            li.classList.add('active');
            sections.forEach(sec => {
                if (sec.id === target) sec.classList.add('active'); else sec.classList.remove('active');
            });
        });
    });
}

window.addEventListener('DOMContentLoaded', () => {
    setupSidebar();
    loadDashboard();
    setInterval(loadDashboard, 1000);
});