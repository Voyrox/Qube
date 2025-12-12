const API_BASE = '/api';

let token = localStorage.getItem('token');
let currentUser = JSON.parse(localStorage.getItem('user') || 'null');

document.addEventListener('DOMContentLoaded', () => {
    initializeAuth();
    loadMostPulled();
    loadTrending();
    setupEventListeners();
});

function initializeAuth() {
    if (token) {
        currentUser = JSON.parse(localStorage.getItem('user') || 'null');
    }
    updateNavbar();
}

function updateNavbar() {
    const navAuth = document.getElementById('navAuth');
    const navUser = document.getElementById('navUser');

    if (!navAuth || !navUser) {
        return; // Page without navbar; skip
    }

    if (token && currentUser) {
        navAuth.style.display = 'none';
        navUser.style.display = 'flex';
        const uname = document.getElementById('profileUsername');
        const uemail = document.getElementById('profileEmail');
        if (uname) uname.textContent = currentUser.username || 'User';
        if (uemail) uemail.textContent = currentUser.email || 'user@example.com';
    } else {
        navAuth.style.display = 'flex';
        navUser.style.display = 'none';
    }
}

function setupEventListeners() {
    const uploadModal = document.getElementById('uploadModal');
    const uploadBtn = document.getElementById('uploadBtn');
    if (uploadBtn && uploadModal) {
        uploadBtn.addEventListener('click', () => {
            uploadModal.style.display = 'block';
        });
    }

    const navToggle = document.getElementById('navToggle');
    const navMenu = document.querySelector('.nav-menu');
    if (navToggle && navMenu) {
        navToggle.addEventListener('click', (e) => {
            e.stopPropagation();
            navMenu.classList.toggle('open');
            navToggle.classList.toggle('open');
        });
        document.addEventListener('click', () => {
            navMenu.classList.remove('open');
            navToggle.classList.remove('open');
        });
        navMenu.addEventListener('click', (e) => e.stopPropagation());
    }

    const profileMenuBtn = document.getElementById('profileMenuBtn');
    const profileDropdown = document.getElementById('profileDropdown');
    if (profileMenuBtn && profileDropdown) {
        profileMenuBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            profileDropdown.classList.toggle('active');
        });
        document.addEventListener('click', () => {
            profileDropdown.classList.remove('active');
        });
    }

    const logoutBtn = document.getElementById('logoutBtn');
    if (logoutBtn) {
        logoutBtn.addEventListener('click', (e) => {
            e.preventDefault();
            logout();
        });
    }

    document.querySelectorAll('.close, .close-modal').forEach(btn => {
        btn.addEventListener('click', (e) => {
            e.preventDefault();
            const modal = btn.closest('.modal');
            if (modal) modal.style.display = 'none';
        });
    });

    window.addEventListener('click', (e) => {
        if (e.target.classList.contains('modal')) {
            e.target.style.display = 'none';
        }
    });

    const uploadForm = document.getElementById('uploadForm');
    if (uploadForm) uploadForm.addEventListener('submit', handleUpload);

    const fileInputs = document.querySelectorAll('input[type="file"]');
    fileInputs.forEach(input => {
        const wrapper = input.parentElement;
        if (wrapper && wrapper.classList.contains('file-input-wrapper')) {
            const label = wrapper.querySelector('.file-input-label');
            label.addEventListener('click', () => input.click());
            wrapper.addEventListener('dragover', (e) => {
                e.preventDefault();
                label.style.borderColor = 'var(--primary)';
                label.style.background = 'rgba(99, 102, 241, 0.05)';
            });
            wrapper.addEventListener('dragleave', () => {
                label.style.borderColor = '';
                label.style.background = '';
            });
            wrapper.addEventListener('drop', (e) => {
                e.preventDefault();
                label.style.borderColor = '';
                label.style.background = '';
                if (e.dataTransfer.files.length) {
                    input.files = e.dataTransfer.files;
                    if (input.files.length) label.textContent = input.files[0].name;
                }
            });
            input.addEventListener('change', () => {
                if (input.files.length) label.textContent = input.files[0].name;
            });
        }
    });

    const searchBtn = document.getElementById('searchBtn');
    const searchInput = document.getElementById('searchInput');
    if (searchBtn) searchBtn.addEventListener('click', handleSearch);
    if (searchInput) {
        searchInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') handleSearch();
        });
    }
}

async function handleUpload(e) {
    e.preventDefault();
    if (!token) {
        showNotification('Please login to upload images', 'error');
        return;
    }

    const submitBtn = e.target.querySelector('button[type="submit"]');
    const originalBtnText = submitBtn.textContent;
    
    // Prevent spam clicking
    if (submitBtn.disabled) return;

    const name = document.getElementById('uploadName').value.trim();
    const tag = document.getElementById('uploadTag').value.trim();
    const description = document.getElementById('uploadDescription').value.trim();
    const file = document.getElementById('uploadFile').files[0];
    const logo = document.getElementById('uploadLogo').files[0];
    const isPublic = document.getElementById('uploadPublic').checked;

    if (!name || !tag || !file) {
        showNotification('Please fill in all required fields', 'error');
        return;
    }
    if (!/^[a-z0-9-]+$/.test(name)) {
        showNotification('Name must be lowercase letters, numbers, hyphens', 'error');
        return;
    }

    // Disable button and show loading state
    submitBtn.disabled = true;
    submitBtn.innerHTML = '<span class="spinner" style="width: 16px; height: 16px; border: 2px solid #fff; border-top-color: transparent; border-radius: 50%; display: inline-block; animation: spin 0.8s linear infinite; margin-right: 8px;"></span>Uploading...';

    try {
        const resp = await fetch(`${API_BASE}/images`);
        const data = await resp.json();
        if (resp.ok && Array.isArray(data.images)) {
            const exists = data.images.some(img => (img.name || '').toLowerCase() === name.toLowerCase());
            if (exists) {
                showNotification('An image with this name already exists', 'error');
                submitBtn.disabled = false;
                submitBtn.textContent = originalBtnText;
                return;
            }
        }
    } catch (err) {
        console.warn('Duplicate check failed:', err);
    }

    const formData = new FormData();
    formData.append('name', name);
    formData.append('tag', tag);
    formData.append('description', description);
    formData.append('file', file);
    if (logo) formData.append('logo', logo);
    formData.append('is_public', isPublic);

    try {
        const response = await fetch(`${API_BASE}/images/upload`, {
            method: 'POST',
            headers: { 'Authorization': `Bearer ${token}` },
            body: formData
        });
        const data = await response.json();
        if (response.ok) {
            console.log('Upload successful:', data);
            showNotification('Image uploaded successfully!', 'success');
            document.getElementById('uploadForm').reset();
            document.getElementById('uploadModal').style.display = 'none';
            loadMostPulled();
            loadTrending();
        } else {
            const errorMsg = data.error || 'Upload failed';
            console.error('Upload failed:', errorMsg);
            showNotification(errorMsg, 'error');
        }
    } catch (error) {
        console.error('Upload error:', error);
        showNotification('Upload error. See console for details.', 'error');
    } finally {
        // Re-enable button
        submitBtn.disabled = false;
        submitBtn.textContent = originalBtnText;
    }
}

function logout() {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    try { document.cookie = 'token=; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT'; } catch {}
    token = null;
    currentUser = null;
    updateNavbar();
    showNotification('Logged out successfully', 'success');
    const profileDropdown = document.getElementById('profileDropdown');
    if (profileDropdown) profileDropdown.classList.remove('active');
}

async function loadMostPulled() {
    const container = document.getElementById('mostPulledList');
    if (!container) return;
    container.innerHTML = '<div class="loading">Loading...</div>';
    try {
        const response = await fetch(`${API_BASE}/images`);
        const data = await response.json();
        if (response.ok && data.images) {
            const topImages = data.images
                .sort((a, b) => (b.stars || 0) - (a.stars || 0))
                .slice(0, 6);
            displayImages(topImages, false, 'mostPulledList');
        } else {
            container.innerHTML = '<div class="loading">No images found</div>';
        }
    } catch (error) {
        console.error('Error loading most pulled:', error);
        container.innerHTML = '<div class="loading">Failed to load</div>';
    }
}

async function loadTrending() {
    const container = document.getElementById('trendingList');
    if (!container) return;
    container.innerHTML = '<div class="loading">Loading...</div>';
    try {
        const response = await fetch(`${API_BASE}/images`);
        const data = await response.json();
        if (response.ok && data.images) {
            // Trending: by most pulls
            const trending = data.images
                .sort((a, b) => (b.pulls || 0) - (a.pulls || 0))
                .slice(0, 6);
            displayImages(trending, false, 'trendingList');
        } else {
            container.innerHTML = '<div class="loading">No images found</div>';
        }
    } catch (error) {
        console.error('Error loading trending:', error);
        container.innerHTML = '<div class="loading">Failed to load</div>';
    }
}

function displayImages(images, showDelete = false, containerId = 'imagesList') {
    const container = document.getElementById(containerId);
    if (!container) return;
    if (!images || images.length === 0) {
        container.innerHTML = '<div class="loading">No images found</div>';
        return;
    }
    container.innerHTML = images.map(image => {
        const tagStr = String(image.tag || '').trim();
        
        // Determine version: use tag if it's a valid version (numbers and dots), otherwise default to 1.0.0
        let version = tagStr;
        if (!tagStr || !/^[0-9.]+$/.test(tagStr)) {
            version = '1.0.0';
        }
        
        // Display version as "# v1.0.0"
        const versionHtml = `<span class="tag"># v${escapeHtml(version)}</span>`;
        
        return `
        <div class="image-card" onclick="viewImage('${escapeHtml(image.name)}', '${escapeHtml(version)}')">
            <div class="image-header">
                ${image.logo_path ? `<img src="${image.logo_path}" alt="${escapeHtml(image.name)} logo" class="image-logo">` : '<div class="image-logo-placeholder">🧊</div>'}
                <div class="image-title">
                    <h3>${escapeHtml((image.owner_username || image.owner_username_fallback || image.owner || 'user'))}/${escapeHtml(image.name)}</h3>
                    <div class="tags">${versionHtml}</div>
                </div>
            </div>
            <p>${escapeHtml((image.description || 'No description')).slice(0, 30)}</p>
            <div class="separator"></div>
            <div class="stats stats-row">
                <span>Pulls: ${image.pulls || 0}</span>
                <span>Stars: ${image.stars || 0}</span>
                <span>Last updated: ${formatDate(image.last_updated || image.updated_at)}</span>
            </div>
            ${showDelete ? `<button class="delete" onclick="event.stopPropagation(); deleteImage('${image.id}')">🗑️ Delete</button>` : ''}
        </div>`;
    }).join('');
}

function viewImage(name, tag) {
    if (!tag) tag = 'latest';
    window.location.href = `/images/${encodeURIComponent(name)}/${encodeURIComponent(tag)}`;
}

async function downloadImage(name, tag) {
    if (!tag) tag = 'latest';
    window.location.href = `${API_BASE}/images/${name}/${tag}/download`;
    showNotification('Download started...', 'success');
}

async function deleteImage(imageId) {
    if (!confirm('Are you sure you want to delete this image?')) return;
    try {
        const response = await fetch(`${API_BASE}/images/${imageId}`, {
            method: 'DELETE',
            headers: { 'Authorization': `Bearer ${token}` }
        });
        if (response.ok) {
            showNotification('Image deleted successfully', 'success');
            loadMostPulled();
            loadTrending();
        } else {
            const data = await response.json();
            showNotification(data.error || 'Failed to delete image', 'error');
        }
    } catch (error) {
        console.error('Error deleting image:', error);
        showNotification('Network error', 'error');
    }
}

async function toggleStar(imageId, event) {
    event.preventDefault();
    if (!token) {
        showNotification('Please login to star images', 'error');
        return;
    }
    const button = event.currentTarget || event.target.closest('button') || event.target;
    const isStarred = button.classList.contains('starred');
    try {
        const response = await fetch(`${API_BASE}/image-id/${imageId}/star`, {
            method: isStarred ? 'DELETE' : 'POST',
            headers: { 'Authorization': `Bearer ${token}` }
        });
        if (response.ok) {
            const data = await response.json().catch(() => ({}));
            button.classList.toggle('starred');
            button.textContent = isStarred ? '☆ Star' : '⭐ Starred';
            const starsEl = document.getElementById('starsCount');
            if (starsEl && typeof data.stars === 'number') {
                starsEl.textContent = String(data.stars);
            }
            showNotification(isStarred ? 'Unstarred' : 'Starred!', 'success');
            setTimeout(() => { loadMostPulled(); loadTrending(); }, 500);
        } else {
            const data = await response.json();
            showNotification(data.error || 'Failed to star image', 'error');
        }
    } catch (error) {
        console.error('Error starring image:', error);
        showNotification('Network error', 'error');
    }
}

function handleSearch() {
    const query = document.getElementById('searchInput').value.trim();
    if (query) {
        showNotification('Search feature coming soon!', 'success');
    }
}

function showNotification(message, type = 'success') {
    const notification = document.createElement('div');
    notification.className = `notification ${type}`;
    notification.textContent = message;
    document.body.appendChild(notification);
    setTimeout(() => {
        notification.style.opacity = '0';
        setTimeout(() => { notification.remove(); }, 300);
    }, 4000);
}

function formatSize(bytes) {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(2) + ' KB';
    if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(2) + ' MB';
    return (bytes / (1024 * 1024 * 1024)).toFixed(2) + ' GB';
}

function escapeHtml(text) {
    const map = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' };
    return String(text).replace(/[&<>"']/g, m => map[m]);
}

function formatDate(dateString) {
    try {
        const date = new Date(dateString);
        const now = new Date();
        const diffMs = now - date;
        const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));
        if (diffDays === 0) return 'Today';
        if (diffDays === 1) return 'Yesterday';
        if (diffDays < 7) return `${diffDays} days ago`;
        if (diffDays < 30) return `${Math.floor(diffDays / 7)} weeks ago`;
        if (diffDays < 365) return `${Math.floor(diffDays / 30)} months ago`;
        return `${Math.floor(diffDays / 365)} years ago`;
    } catch {
        return 'Unknown';
    }
}

async function handleRegister(e) {
    e.preventDefault();
    const username = document.getElementById('registerUsername').value;
    const email = document.getElementById('registerEmail').value;
    const password = document.getElementById('registerPassword').value;

    try {
        const response = await fetch(`${API_BASE}/auth/register`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ username, email, password })
        });

        const data = await response.json();

        if (response.ok) {
            token = data.token;
            currentUser = data.user;
            localStorage.setItem('token', token);
            localStorage.setItem('user', JSON.stringify(data.user));
            try { document.cookie = `token=${token}; path=/; SameSite=Lax`; } catch {}
            document.getElementById('registerModal').style.display = 'none';
            updateNavbar();
            showNotification('Registration successful!', 'success');
            loadMostPulled();
            loadTrending();
        } else {
            showNotification(data.error || 'Registration failed', 'error');
        }
    } catch (error) {
        showNotification('Network error', 'error');
    }
}

async function handleLogin(e) {
    e.preventDefault();
    const identifier = document.getElementById('loginUsername').value;
    const password = document.getElementById('loginPassword').value;

    try {
        const response = await fetch(`${API_BASE}/auth/login`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ identifier, password })
        });

        const data = await response.json();

        if (response.ok) {
            token = data.token;
            currentUser = data.user;
            localStorage.setItem('token', token);
            localStorage.setItem('user', JSON.stringify(data.user));
            try {
                document.cookie = `token=${token}; path=/; SameSite=Lax`;
            } catch {}
            document.getElementById('loginModal').style.display = 'none';
            updateNavbar();
            showNotification('Login successful!', 'success');
            loadMostPulled();
            loadTrending();
        } else {
            showNotification(data.error || 'Login failed', 'error');
        }
    } catch (error) {
        showNotification('Network error', 'error');
    }
}