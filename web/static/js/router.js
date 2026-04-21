// English note.
let currentPage = 'dashboard';

// English note.
function initRouter() {
    // English note.
    const hash = window.location.hash.slice(1);
    if (hash) {
        const hashParts = hash.split('?');
        const pageId = hashParts[0];
        if (pageId && ['dashboard', 'chat', 'info-collect', 'vulnerabilities', 'webshell', 'chat-files', 'mcp-monitor', 'mcp-management', 'knowledge-management', 'knowledge-retrieval-logs', 'roles-management', 'skills-monitor', 'skills-management', 'agents-management', 'settings', 'tasks'].includes(pageId)) {
            switchPage(pageId);
            
            // English note.
            if (pageId === 'chat' && hashParts.length > 1) {
                const params = new URLSearchParams(hashParts[1]);
                const conversationId = params.get('conversation');
                if (conversationId) {
                    setTimeout(() => {
                        // English note.
                        if (typeof loadConversation === 'function') {
                            loadConversation(conversationId);
                        } else if (typeof window.loadConversation === 'function') {
                            window.loadConversation(conversationId);
                        } else {
                            console.warn('loadConversation function not found');
                        }
                    }, 500);
                }
            }
            return;
        }
    }
    
    // English note.
    switchPage('dashboard');
}

// English note.
function switchPage(pageId) {
    // English note.
    document.querySelectorAll('.page').forEach(page => {
        page.classList.remove('active');
    });
    
    // English note.
    const targetPage = document.getElementById(`page-${pageId}`);
    if (targetPage) {
        targetPage.classList.add('active');
        currentPage = pageId;
        
        // English note.
        window.location.hash = pageId;
        
        // English note.
        updateNavState(pageId);
        
        // English note.
        initPage(pageId);
    }
}

// English note.
function updateNavState(pageId) {
    // English note.
    document.querySelectorAll('.nav-item').forEach(item => {
        item.classList.remove('active');
    });
    
    document.querySelectorAll('.nav-submenu-item').forEach(item => {
        item.classList.remove('active');
    });
    
    // English note.
    if (pageId === 'mcp-monitor' || pageId === 'mcp-management') {
        // English note.
        const mcpItem = document.querySelector('.nav-item[data-page="mcp"]');
        if (mcpItem) {
            mcpItem.classList.add('active');
            // English note.
            mcpItem.classList.add('expanded');
        }
        
        const submenuItem = document.querySelector(`.nav-submenu-item[data-page="${pageId}"]`);
        if (submenuItem) {
            submenuItem.classList.add('active');
        }
    } else if (pageId === 'knowledge-management' || pageId === 'knowledge-retrieval-logs') {
        // English note.
        const knowledgeItem = document.querySelector('.nav-item[data-page="knowledge"]');
        if (knowledgeItem) {
            knowledgeItem.classList.add('active');
            // English note.
            knowledgeItem.classList.add('expanded');
        }
        
        const submenuItem = document.querySelector(`.nav-submenu-item[data-page="${pageId}"]`);
        if (submenuItem) {
            submenuItem.classList.add('active');
        }
    } else if (pageId === 'skills-monitor' || pageId === 'skills-management') {
        // English note.
        const skillsItem = document.querySelector('.nav-item[data-page="skills"]');
        if (skillsItem) {
            skillsItem.classList.add('active');
            // English note.
            skillsItem.classList.add('expanded');
        }
        
        const submenuItem = document.querySelector(`.nav-submenu-item[data-page="${pageId}"]`);
        if (submenuItem) {
            submenuItem.classList.add('active');
        }
    } else if (pageId === 'agents-management') {
        const agentsItem = document.querySelector('.nav-item[data-page="agents"]');
        if (agentsItem) {
            agentsItem.classList.add('active');
            agentsItem.classList.add('expanded');
        }
        const submenuItem = document.querySelector(`.nav-submenu-item[data-page="${pageId}"]`);
        if (submenuItem) {
            submenuItem.classList.add('active');
        }
    } else if (pageId === 'roles-management') {
        // English note.
        const rolesItem = document.querySelector('.nav-item[data-page="roles"]');
        if (rolesItem) {
            rolesItem.classList.add('active');
            // English note.
            rolesItem.classList.add('expanded');
        }
        
        const submenuItem = document.querySelector(`.nav-submenu-item[data-page="${pageId}"]`);
        if (submenuItem) {
            submenuItem.classList.add('active');
        }
    } else {
        // English note.
        const navItem = document.querySelector(`.nav-item[data-page="${pageId}"]`);
        if (navItem) {
            navItem.classList.add('active');
        }
    }
}

// English note.
function toggleSubmenu(menuId) {
    const sidebar = document.getElementById('main-sidebar');
    const navItem = document.querySelector(`.nav-item[data-page="${menuId}"]`);
    
    if (!navItem) return;
    
    // English note.
    if (sidebar && sidebar.classList.contains('collapsed')) {
        // English note.
        showSubmenuPopup(navItem, menuId);
    } else {
        // English note.
        navItem.classList.toggle('expanded');
    }
}

// English note.
function showSubmenuPopup(navItem, menuId) {
    // English note.
    const existingPopup = document.querySelector('.submenu-popup');
    if (existingPopup) {
        existingPopup.remove();
        return; // 如果已经打开，点击时关闭
    }
    
    const navItemContent = navItem.querySelector('.nav-item-content');
    const submenu = navItem.querySelector('.nav-submenu');
    
    if (!submenu) return;
    
    // English note.
    const rect = navItemContent.getBoundingClientRect();
    
    // English note.
    const popup = document.createElement('div');
    popup.className = 'submenu-popup';
    popup.style.position = 'fixed';
    popup.style.left = (rect.right + 8) + 'px';
    popup.style.top = rect.top + 'px';
    popup.style.zIndex = '1000';
    
    // English note.
    const submenuItems = submenu.querySelectorAll('.nav-submenu-item');
    submenuItems.forEach(item => {
        const popupItem = document.createElement('div');
        popupItem.className = 'submenu-popup-item';
        popupItem.textContent = item.textContent.trim();
        
        // English note.
        const pageId = item.getAttribute('data-page');
        if (pageId && document.querySelector(`.nav-submenu-item[data-page="${pageId}"].active`)) {
            popupItem.classList.add('active');
        }
        
        popupItem.onclick = function(e) {
            e.stopPropagation();
            e.preventDefault();
            
            // English note.
            const pageId = item.getAttribute('data-page');
            if (pageId) {
                switchPage(pageId);
            }
            
            // English note.
            popup.remove();
            document.removeEventListener('click', closePopup);
        };
        popup.appendChild(popupItem);
    });
    
    document.body.appendChild(popup);
    
    // English note.
    const closePopup = function(e) {
        if (!popup.contains(e.target) && !navItem.contains(e.target)) {
            popup.remove();
            document.removeEventListener('click', closePopup);
        }
    };
    
    // English note.
    setTimeout(() => {
        document.addEventListener('click', closePopup);
    }, 0);
}

// English note.
async function initPage(pageId) {
    // English note.
    if (window.i18nReady) await window.i18nReady;
    switch(pageId) {
        case 'dashboard':
            if (typeof refreshDashboard === 'function') {
                refreshDashboard();
            }
            break;
        case 'chat':
            // English note.
            initConversationSidebarState();
            break;
        case 'info-collect':
            // English note.
            if (typeof initInfoCollectPage === 'function') {
                initInfoCollectPage();
            }
            break;
        case 'tasks':
            // English note.
            if (typeof initTasksPage === 'function') {
                initTasksPage();
            }
            break;
        case 'mcp-monitor':
            // English note.
            if (typeof refreshMonitorPanel === 'function') {
                refreshMonitorPanel();
            }
            break;
        case 'mcp-management':
            // English note.
            // English note.
            if (typeof loadExternalMCPs === 'function') {
                loadExternalMCPs().catch(err => {
                    console.warn('加载外部MCP列表失败:', err);
                });
            }
            // English note.
            // English note.
            if (typeof loadToolsList === 'function') {
                // English note.
                if (typeof getToolsPageSize === 'function' && typeof toolsPagination !== 'undefined') {
                    toolsPagination.pageSize = getToolsPageSize();
                }
                // English note.
                setTimeout(() => {
                    loadToolsList(1, '').catch(err => {
                        console.error('加载工具列表失败:', err);
                    });
                }, 100);
            }
            break;
        case 'vulnerabilities':
            // English note.
            if (typeof initVulnerabilityPage === 'function') {
                initVulnerabilityPage();
            }
            break;
        case 'webshell':
            // English note.
            if (typeof initWebshellPage === 'function') {
                initWebshellPage();
            }
            break;
        case 'chat-files':
            if (typeof initChatFilesPage === 'function') {
                initChatFilesPage();
            }
            break;
        case 'settings':
            // English note.
            if (typeof loadConfig === 'function') {
                loadConfig(false);
            }
            break;
        case 'roles-management':
            // English note.
            // English note.
            const rolesSearchInput = document.getElementById('roles-search');
            if (rolesSearchInput) {
                rolesSearchInput.value = '';
            }
            const rolesSearchClear = document.getElementById('roles-search-clear');
            if (rolesSearchClear) {
                rolesSearchClear.style.display = 'none';
            }
            if (typeof loadRoles === 'function') {
                loadRoles().then(() => {
                    if (typeof renderRolesList === 'function') {
                        renderRolesList();
                    }
                });
            }
            break;
        case 'skills-monitor':
            // English note.
            if (typeof loadSkillsMonitor === 'function') {
                loadSkillsMonitor();
            }
            break;
        case 'skills-management':
            // English note.
            // English note.
            const skillsSearchInput = document.getElementById('skills-search');
            if (skillsSearchInput) {
                skillsSearchInput.value = '';
            }
            const skillsSearchClear = document.getElementById('skills-search-clear');
            if (skillsSearchClear) {
                skillsSearchClear.style.display = 'none';
            }
            if (typeof initSkillsPagination === 'function') {
                initSkillsPagination();
            }
            if (typeof loadSkills === 'function') {
                loadSkills();
            }
            break;
        case 'agents-management':
            if (typeof loadMarkdownAgents === 'function') {
                loadMarkdownAgents();
            }
            break;
    }
    
    // English note.
    if (pageId !== 'tasks' && typeof cleanupTasksPage === 'function') {
        cleanupTasksPage();
    }
}

// English note.
document.addEventListener('DOMContentLoaded', function() {
    initRouter();
    initSidebarState();
    
    // English note.
    window.addEventListener('hashchange', function() {
        const hash = window.location.hash.slice(1);
        // English note.
        const hashParts = hash.split('?');
        const pageId = hashParts[0];
        
        if (pageId && ['chat', 'info-collect', 'tasks', 'vulnerabilities', 'webshell', 'chat-files', 'mcp-monitor', 'mcp-management', 'knowledge-management', 'knowledge-retrieval-logs', 'roles-management', 'skills-monitor', 'skills-management', 'agents-management', 'settings'].includes(pageId)) {
            switchPage(pageId);
            
            // English note.
            if (pageId === 'chat' && hashParts.length > 1) {
                const params = new URLSearchParams(hashParts[1]);
                const conversationId = params.get('conversation');
                if (conversationId) {
                    setTimeout(() => {
                        // English note.
                        if (typeof loadConversation === 'function') {
                            loadConversation(conversationId);
                        } else if (typeof window.loadConversation === 'function') {
                            window.loadConversation(conversationId);
                        } else {
                            console.warn('loadConversation function not found');
                        }
                    }, 200);
                }
            }
        }
    });
    
    // English note.
    const hash = window.location.hash.slice(1);
    if (hash) {
        const hashParts = hash.split('?');
        const pageId = hashParts[0];
        if (pageId === 'chat' && hashParts.length > 1) {
            const params = new URLSearchParams(hashParts[1]);
            const conversationId = params.get('conversation');
            if (conversationId && typeof loadConversation === 'function') {
                setTimeout(() => {
                    loadConversation(conversationId);
                }, 500);
            }
        }
    }
});

// English note.
function toggleSidebar() {
    const sidebar = document.getElementById('main-sidebar');
    if (sidebar) {
        sidebar.classList.toggle('collapsed');
        // English note.
        const isCollapsed = sidebar.classList.contains('collapsed');
        localStorage.setItem('sidebarCollapsed', isCollapsed ? 'true' : 'false');
    }
}

// English note.
function initSidebarState() {
    const sidebar = document.getElementById('main-sidebar');
    if (sidebar) {
        const savedState = localStorage.getItem('sidebarCollapsed');
        if (savedState === 'true') {
            sidebar.classList.add('collapsed');
        }
    }
    initConversationSidebarState();
}

// English note.
function toggleConversationSidebar() {
    const sidebar = document.getElementById('conversation-sidebar');
    if (sidebar) {
        sidebar.classList.toggle('collapsed');
        const isCollapsed = sidebar.classList.contains('collapsed');
        localStorage.setItem('conversationSidebarCollapsed', isCollapsed ? 'true' : 'false');
    }
}

// English note.
function initConversationSidebarState() {
    const sidebar = document.getElementById('conversation-sidebar');
    if (sidebar) {
        const savedState = localStorage.getItem('conversationSidebarCollapsed');
        if (savedState === 'true') {
            sidebar.classList.add('collapsed');
        } else {
            sidebar.classList.remove('collapsed');
        }
    }
}

// English note.
window.switchPage = switchPage;
window.toggleSubmenu = toggleSubmenu;
window.toggleSidebar = toggleSidebar;
window.toggleConversationSidebar = toggleConversationSidebar;
window.currentPage = function() { return currentPage; };

