// English note.
function _t(key, opts) {
    return typeof window.t === 'function' ? window.t(key, opts) : key;
}
let currentRole = localStorage.getItem('currentRole') || '';
let roles = [];
let rolesSearchKeyword = ''; // 
let rolesSearchTimeout = null; // 
let allRoleTools = []; // （）
let roleToolsPagination = {
    page: 1,
    pageSize: 20,
    total: 0,
    totalPages: 1
};
let roleToolsSearchKeyword = ''; // 
let roleToolStateMap = new Map(); // ：toolKey -> { enabled: boolean, ... }
let roleUsesAllTools = false; // （tools）
let totalEnabledToolsInMCP = 0; // （MCP，API）
let roleConfiguredTools = new Set(); // （）

// English note.
let allRoleSkills = []; // skills
let roleSkillsSearchKeyword = ''; // Skills
let roleSelectedSkills = new Set(); // skills

// English note.
function sortRoles(rolesArray) {
    const sortedRoles = [...rolesArray];
    // English note.
    const defaultRole = sortedRoles.find(r => r.name === '');
    const otherRoles = sortedRoles.filter(r => r.name !== '');
    
    // English note.
    otherRoles.sort((a, b) => {
        const nameA = a.name || '';
        const nameB = b.name || '';
        return nameA.localeCompare(nameB, 'zh-CN');
    });
    
    // English note.
    const result = defaultRole ? [defaultRole, ...otherRoles] : otherRoles;
    return result;
}

// English note.
async function loadRoles() {
    try {
        const response = await apiFetch('/api/roles');
        if (!response.ok) {
            throw new Error('');
        }
        const data = await response.json();
        roles = data.roles || [];
        updateRoleSelectorDisplay();
        renderRoleSelectionSidebar(); // 
        return roles;
    } catch (error) {
        console.error(':', error);
        // English note.
        var loadFailedLabel = (typeof window !== 'undefined' && typeof window.t === 'function')
            ? window.t('roles.loadFailed')
            : '';
        showNotification(loadFailedLabel + ': ' + error.message, 'error');
        return [];
    }
}

// English note.
function handleRoleChange(roleName) {
    const oldRole = currentRole;
    currentRole = roleName || '';
    localStorage.setItem('currentRole', currentRole);
    updateRoleSelectorDisplay();
    renderRoleSelectionSidebar(); // 
    
    // English note.
    // English note.
    if (oldRole !== currentRole && typeof window !== 'undefined') {
        // English note.
        window._mentionToolsRoleChanged = true;
    }
}

// English note.
function updateRoleSelectorDisplay() {
    const roleSelectorBtn = document.getElementById('role-selector-btn');
    const roleSelectorIcon = document.getElementById('role-selector-icon');
    const roleSelectorText = document.getElementById('role-selector-text');
    
    if (!roleSelectorBtn || !roleSelectorIcon || !roleSelectorText) return;

    let selectedRole;
    if (currentRole && currentRole !== '') {
        selectedRole = roles.find(r => r.name === currentRole);
    } else {
        selectedRole = roles.find(r => r.name === '');
    }

    if (selectedRole) {
        // English note.
        let icon = selectedRole.icon || '🔵';
        // English note.
        if (icon && typeof icon === 'string') {
            const unicodeMatch = icon.match(/^"?\\U([0-9A-F]{8})"?$/i);
            if (unicodeMatch) {
                try {
                    const codePoint = parseInt(unicodeMatch[1], 16);
                    icon = String.fromCodePoint(codePoint);
                } catch (e) {
                    // English note.
                    console.warn(' icon Unicode :', icon, e);
                    icon = '🔵';
                }
            }
        }
        roleSelectorIcon.textContent = icon;
        const isDefaultRole = selectedRole.name === '' || !selectedRole.name;
        const displayName = isDefaultRole && typeof window.t === 'function'
            ? window.t('chat.defaultRole') : (selectedRole.name || (typeof window.t === 'function' ? window.t('chat.defaultRole') : ''));
        // English note.
        roleSelectorText.setAttribute('data-i18n-skip-text', isDefaultRole ? 'false' : 'true');
        roleSelectorText.textContent = displayName;
    } else {
        // English note.
        roleSelectorText.setAttribute('data-i18n-skip-text', 'false');
        roleSelectorIcon.textContent = '🔵';
        roleSelectorText.textContent = typeof window.t === 'function' ? window.t('chat.defaultRole') : '';
    }
}

// English note.
function renderRoleSelectionSidebar() {
    const roleList = document.getElementById('role-selection-list');
    if (!roleList) return;

    // English note.
    roleList.innerHTML = '';

    // English note.
    function getRoleIcon(role) {
        if (role.icon) {
            // English note.
            let icon = role.icon;
            // English note.
            const unicodeMatch = icon.match(/^"?\\U([0-9A-F]{8})"?$/i);
            if (unicodeMatch) {
                try {
                    const codePoint = parseInt(unicodeMatch[1], 16);
                    icon = String.fromCodePoint(codePoint);
                } catch (e) {
                    // English note.
                    console.warn(' icon Unicode :', icon, e);
                }
            }
            return icon;
        }
        // English note.
        // English note.
        return '👤';
    }
    
    // English note.
    const sortedRoles = sortRoles(roles);
    
    // English note.
    const enabledSortedRoles = sortedRoles.filter(r => r.enabled !== false);
    
    enabledSortedRoles.forEach(role => {
        const isDefaultRole = role.name === '';
        const isSelected = isDefaultRole ? (currentRole === '' || currentRole === '') : (currentRole === role.name);
        const roleItem = document.createElement('div');
        roleItem.className = 'role-selection-item-main' + (isSelected ? ' selected' : '');
        roleItem.onclick = () => {
            selectRole(role.name);
            closeRoleSelectionPanel(); // 
        };
        const icon = getRoleIcon(role);
        
        // English note.
        let description = role.description || _t('roles.noDescription');
        if (isDefaultRole && !role.description) {
            description = _t('roles.defaultRoleDescription');
        }
        
        roleItem.innerHTML = `
            <div class="role-selection-item-icon-main">${icon}</div>
            <div class="role-selection-item-content-main">
                <div class="role-selection-item-name-main">${escapeHtml(role.name)}</div>
                <div class="role-selection-item-description-main">${escapeHtml(description)}</div>
            </div>
            ${isSelected ? '<div class="role-selection-checkmark-main">✓</div>' : ''}
        `;
        roleList.appendChild(roleItem);
    });
}

// English note.
function selectRole(roleName) {
    // English note.
    if (roleName === '') {
        roleName = '';
    }
    handleRoleChange(roleName);
    renderRoleSelectionSidebar(); // 
}

// English note.
function toggleRoleSelectionPanel() {
    const panel = document.getElementById('role-selection-panel');
    const roleSelectorBtn = document.getElementById('role-selector-btn');
    if (!panel) return;
    
    const isHidden = panel.style.display === 'none' || !panel.style.display;
    
    if (isHidden) {
        if (typeof closeAgentModePanel === 'function') {
            closeAgentModePanel();
        }
        panel.style.display = 'flex'; // flex
        // English note.
        if (roleSelectorBtn) {
            roleSelectorBtn.classList.add('active');
        }
        
        // English note.
        setTimeout(() => {
            const wrapper = document.querySelector('.role-selector-wrapper');
            if (wrapper) {
                const rect = wrapper.getBoundingClientRect();
                const panelHeight = panel.offsetHeight || 400;
                const viewportHeight = window.innerHeight;
                
                // English note.
                if (rect.top - panelHeight < 0) {
                    const scrollY = window.scrollY + rect.top - panelHeight - 20;
                    window.scrollTo({ top: Math.max(0, scrollY), behavior: 'smooth' });
                }
            }
        }, 10);
    } else {
        panel.style.display = 'none';
        // English note.
        if (roleSelectorBtn) {
            roleSelectorBtn.classList.remove('active');
        }
    }
}

// English note.
function closeRoleSelectionPanel() {
    const panel = document.getElementById('role-selection-panel');
    const roleSelectorBtn = document.getElementById('role-selector-btn');
    if (panel) {
        panel.style.display = 'none';
    }
    if (roleSelectorBtn) {
        roleSelectorBtn.classList.remove('active');
    }
}

// English note.
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// English note.
async function refreshRoles() {
    await loadRoles();
    // English note.
    const currentPage = typeof window.currentPage === 'function' ? window.currentPage() : (window.currentPage || 'chat');
    if (currentPage === 'roles-management') {
        renderRolesList();
    }
    // English note.
    renderRoleSelectionSidebar();
    showNotification('', 'success');
}

// English note.
function renderRolesList() {
    const rolesList = document.getElementById('roles-list');
    if (!rolesList) return;

    // English note.
    let filteredRoles = roles;
    if (rolesSearchKeyword) {
        const keyword = rolesSearchKeyword.toLowerCase();
        filteredRoles = roles.filter(role => 
            role.name.toLowerCase().includes(keyword) ||
            (role.description && role.description.toLowerCase().includes(keyword))
        );
    }

    if (filteredRoles.length === 0) {
        rolesList.innerHTML = '<div class="empty-state">' + 
            (rolesSearchKeyword ? _t('roles.noMatchingRoles') : _t('roles.noRoles')) + 
            '</div>';
        return;
    }

    // English note.
    const sortedRoles = sortRoles(filteredRoles);
    
    rolesList.innerHTML = sortedRoles.map(role => {
        // English note.
        let roleIcon = role.icon || '👤';
        if (roleIcon && typeof roleIcon === 'string') {
            // English note.
            const unicodeMatch = roleIcon.match(/^"?\\U([0-9A-F]{8})"?$/i);
            if (unicodeMatch) {
                try {
                    const codePoint = parseInt(unicodeMatch[1], 16);
                    roleIcon = String.fromCodePoint(codePoint);
                } catch (e) {
                    // English note.
                    console.warn(' icon Unicode :', roleIcon, e);
                    roleIcon = '👤';
                }
            }
        }

        // English note.
        let toolsDisplay = '';
        let toolsCount = 0;
        if (role.name === '') {
            toolsDisplay = _t('roleModal.usingAllTools');
        } else if (role.tools && role.tools.length > 0) {
            toolsCount = role.tools.length;
            // English note.
            const toolNames = role.tools.slice(0, 5).map(tool => {
                // English note.
                const toolName = tool.includes('::') ? tool.split('::')[1] : tool;
                return escapeHtml(toolName);
            });
            if (toolsCount <= 5) {
                toolsDisplay = toolNames.join(', ');
            } else {
                toolsDisplay = toolNames.join(', ') + _t('roleModal.andNMore', { count: toolsCount });
            }
        } else if (role.mcps && role.mcps.length > 0) {
            toolsCount = role.mcps.length;
            toolsDisplay = _t('roleModal.andNMore', { count: toolsCount });
        } else {
            toolsDisplay = _t('roleModal.usingAllTools');
        }

        return `
        <div class="role-card">
            <div class="role-card-header">
                <h3 class="role-card-title">
                    <span class="role-card-icon">${roleIcon}</span>
                    ${escapeHtml(role.name)}
                </h3>
                <span class="role-card-badge ${role.enabled !== false ? 'enabled' : 'disabled'}">
                    ${role.enabled !== false ? _t('roles.enabled') : _t('roles.disabled')}
                </span>
            </div>
            <div class="role-card-description">${escapeHtml(role.description || _t('roles.noDescriptionShort'))}</div>
            <div class="role-card-tools">
                <span class="role-card-tools-label">${_t('roleModal.toolsLabel')}</span>
                <span class="role-card-tools-value">${toolsDisplay}</span>
            </div>
            <div class="role-card-actions">
                <button class="btn-secondary btn-small" onclick="editRole('${escapeHtml(role.name)}')">${_t('common.edit')}</button>
                ${role.name !== '' ? `<button class="btn-secondary btn-small btn-danger" onclick="deleteRole('${escapeHtml(role.name)}')">${_t('common.delete')}</button>` : ''}
            </div>
        </div>
    `;
    }).join('');
}

// English note.
function handleRolesSearchInput() {
    clearTimeout(rolesSearchTimeout);
    rolesSearchTimeout = setTimeout(() => {
        searchRoles();
    }, 300);
}

// English note.
function searchRoles() {
    const searchInput = document.getElementById('roles-search');
    if (!searchInput) return;
    
    rolesSearchKeyword = searchInput.value.trim();
    const clearBtn = document.getElementById('roles-search-clear');
    if (clearBtn) {
        clearBtn.style.display = rolesSearchKeyword ? 'block' : 'none';
    }
    
    renderRolesList();
}

// English note.
function clearRolesSearch() {
    const searchInput = document.getElementById('roles-search');
    if (searchInput) {
        searchInput.value = '';
    }
    rolesSearchKeyword = '';
    const clearBtn = document.getElementById('roles-search-clear');
    if (clearBtn) {
        clearBtn.style.display = 'none';
    }
    renderRolesList();
}

// English note.
function getToolKey(tool) {
    // English note.
    if (tool.is_external && tool.external_mcp) {
        return `${tool.external_mcp}::${tool.name}`;
    }
    // English note.
    return tool.name;
}

// English note.
function saveCurrentRolePageToolStates() {
    document.querySelectorAll('#role-tools-list .role-tool-item').forEach(item => {
        const toolKey = item.dataset.toolKey;
        const checkbox = item.querySelector('input[type="checkbox"]');
        if (toolKey && checkbox) {
            const toolName = item.dataset.toolName;
            const isExternal = item.dataset.isExternal === 'true';
            const externalMcp = item.dataset.externalMcp || '';
            const existingState = roleToolStateMap.get(toolKey);
            roleToolStateMap.set(toolKey, {
                enabled: checkbox.checked,
                is_external: isExternal,
                external_mcp: externalMcp,
                name: toolName,
                mcpEnabled: existingState ? existingState.mcpEnabled : true // MCP
            });
        }
    });
}

// English note.
async function loadRoleTools(page = 1, searchKeyword = '') {
    try {
        // English note.
        saveCurrentRolePageToolStates();
        
        const pageSize = roleToolsPagination.pageSize;
        let url = `/api/config/tools?page=${page}&page_size=${pageSize}`;
        if (searchKeyword) {
            url += `&search=${encodeURIComponent(searchKeyword)}`;
        }
        
        const response = await apiFetch(url);
        if (!response.ok) {
            throw new Error('');
        }
        
        const result = await response.json();
        allRoleTools = result.tools || [];
        roleToolsPagination = {
            page: result.page || page,
            pageSize: result.page_size || pageSize,
            total: result.total || 0,
            totalPages: result.total_pages || 1
        };
        
        // English note.
        if (result.total_enabled !== undefined) {
            totalEnabledToolsInMCP = result.total_enabled;
        }
        
        // English note.
        // English note.
        allRoleTools.forEach(tool => {
            const toolKey = getToolKey(tool);
            if (!roleToolStateMap.has(toolKey)) {
                // English note.
                let enabled = false;
                if (roleUsesAllTools) {
                    // English note.
                    enabled = tool.enabled ? true : false;
                } else {
                    // English note.
                    enabled = roleConfiguredTools.has(toolKey);
                }
                roleToolStateMap.set(toolKey, {
                    enabled: enabled,
                    is_external: tool.is_external || false,
                    external_mcp: tool.external_mcp || '',
                    name: tool.name,
                    mcpEnabled: tool.enabled // MCP
                });
            } else {
                // English note.
                // English note.
                const state = roleToolStateMap.get(toolKey);
                // English note.
                if (roleUsesAllTools && tool.enabled) {
                    // English note.
                    state.enabled = true;
                }
                // English note.
                state.is_external = tool.is_external || false;
                state.external_mcp = tool.external_mcp || '';
                state.mcpEnabled = tool.enabled; // MCP
                if (!state.name || state.name === toolKey.split('::').pop()) {
                    state.name = tool.name; // 
                }
            }
        });
        
        renderRoleToolsList();
        renderRoleToolsPagination();
        updateRoleToolsStats();
    } catch (error) {
        console.error(':', error);
        const toolsList = document.getElementById('role-tools-list');
        if (toolsList) {
            toolsList.innerHTML = `<div class="tools-error">${_t('roleModal.loadToolsFailed')}: ${escapeHtml(error.message)}</div>`;
        }
    }
}

// English note.
function renderRoleToolsList() {
    const toolsList = document.getElementById('role-tools-list');
    if (!toolsList) return;
    
    // English note.
    toolsList.innerHTML = '';
    
    const listContainer = document.createElement('div');
    listContainer.className = 'role-tools-list-items';
    listContainer.innerHTML = '';
    
    if (allRoleTools.length === 0) {
        listContainer.innerHTML = '<div class="tools-empty">' + _t('roleModal.noTools') + '</div>';
        toolsList.appendChild(listContainer);
        return;
    }
    
    allRoleTools.forEach(tool => {
        const toolKey = getToolKey(tool);
        const toolItem = document.createElement('div');
        toolItem.className = 'role-tool-item';
        toolItem.dataset.toolKey = toolKey;
        toolItem.dataset.toolName = tool.name;
        toolItem.dataset.isExternal = tool.is_external ? 'true' : 'false';
        toolItem.dataset.externalMcp = tool.external_mcp || '';
        
        // English note.
        const toolState = roleToolStateMap.get(toolKey) || {
            enabled: tool.enabled,
            is_external: tool.is_external || false,
            external_mcp: tool.external_mcp || ''
        };
        
        // English note.
        let externalBadge = '';
        if (toolState.is_external || tool.is_external) {
            const externalMcpName = toolState.external_mcp || tool.external_mcp || '';
            const badgeText = externalMcpName ? ` (${escapeHtml(externalMcpName)})` : '';
            const badgeTitle = externalMcpName ? `MCP - ：${escapeHtml(externalMcpName)}` : 'MCP';
            externalBadge = `<span class="external-tool-badge" title="${badgeTitle}">${badgeText}</span>`;
        }
        
        // English note.
        const checkboxId = `role-tool-${escapeHtml(toolKey).replace(/::/g, '--')}`;
        
        toolItem.innerHTML = `
            <input type="checkbox" id="${checkboxId}" ${toolState.enabled ? 'checked' : ''} 
                   onchange="handleRoleToolCheckboxChange('${escapeHtml(toolKey)}', this.checked)" />
            <div class="role-tool-item-info">
                <div class="role-tool-item-name">
                    ${escapeHtml(tool.name)}
                    ${externalBadge}
                </div>
                <div class="role-tool-item-desc">${escapeHtml(tool.description || '')}</div>
            </div>
        `;
        listContainer.appendChild(toolItem);
    });
    
    toolsList.appendChild(listContainer);
}

// English note.
function renderRoleToolsPagination() {
    const toolsList = document.getElementById('role-tools-list');
    if (!toolsList) return;
    
    // English note.
    const oldPagination = toolsList.querySelector('.role-tools-pagination');
    if (oldPagination) {
        oldPagination.remove();
    }
    
    // English note.
    if (roleToolsPagination.totalPages <= 1) {
        return;
    }
    
    const pagination = document.createElement('div');
    pagination.className = 'role-tools-pagination';
    
    const { page, totalPages, total } = roleToolsPagination;
    const startItem = (page - 1) * roleToolsPagination.pageSize + 1;
    const endItem = Math.min(page * roleToolsPagination.pageSize, total);
    
    const paginationShowText = _t('roleModal.paginationShow', { start: startItem, end: endItem, total: total }) +
        (roleToolsSearchKeyword ? _t('roleModal.paginationSearch', { keyword: roleToolsSearchKeyword }) : '');
    pagination.innerHTML = `
        <div class="pagination-info">${paginationShowText}</div>
        <div class="pagination-controls">
            <button class="btn-secondary" onclick="loadRoleTools(1, '${escapeHtml(roleToolsSearchKeyword)}')" ${page === 1 ? 'disabled' : ''}>${_t('roleModal.firstPage')}</button>
            <button class="btn-secondary" onclick="loadRoleTools(${page - 1}, '${escapeHtml(roleToolsSearchKeyword)}')" ${page === 1 ? 'disabled' : ''}>${_t('roleModal.prevPage')}</button>
            <span class="pagination-page">${_t('roleModal.pageOf', { page: page, total: totalPages })}</span>
            <button class="btn-secondary" onclick="loadRoleTools(${page + 1}, '${escapeHtml(roleToolsSearchKeyword)}')" ${page === totalPages ? 'disabled' : ''}>${_t('roleModal.nextPage')}</button>
            <button class="btn-secondary" onclick="loadRoleTools(${totalPages}, '${escapeHtml(roleToolsSearchKeyword)}')" ${page === totalPages ? 'disabled' : ''}>${_t('roleModal.lastPage')}</button>
        </div>
    `;
    
    toolsList.appendChild(pagination);
}

// English note.
function handleRoleToolCheckboxChange(toolKey, enabled) {
    const toolItem = document.querySelector(`.role-tool-item[data-tool-key="${toolKey}"]`);
    if (toolItem) {
        const toolName = toolItem.dataset.toolName;
        const isExternal = toolItem.dataset.isExternal === 'true';
        const externalMcp = toolItem.dataset.externalMcp || '';
        const existingState = roleToolStateMap.get(toolKey);
        roleToolStateMap.set(toolKey, {
            enabled: enabled,
            is_external: isExternal,
            external_mcp: externalMcp,
            name: toolName,
            mcpEnabled: existingState ? existingState.mcpEnabled : true // MCP
        });
    }
    updateRoleToolsStats();
}

// English note.
function selectAllRoleTools() {
    document.querySelectorAll('#role-tools-list input[type="checkbox"]').forEach(checkbox => {
        const toolItem = checkbox.closest('.role-tool-item');
        if (toolItem) {
            const toolKey = toolItem.dataset.toolKey;
            const toolName = toolItem.dataset.toolName;
            const isExternal = toolItem.dataset.isExternal === 'true';
            const externalMcp = toolItem.dataset.externalMcp || '';
            if (toolKey) {
                const existingState = roleToolStateMap.get(toolKey);
                // English note.
                const shouldEnable = existingState && existingState.mcpEnabled !== false;
                checkbox.checked = shouldEnable;
                roleToolStateMap.set(toolKey, {
                    enabled: shouldEnable,
                    is_external: isExternal,
                    external_mcp: externalMcp,
                    name: toolName,
                    mcpEnabled: existingState ? existingState.mcpEnabled : true
                });
            }
        }
    });
    updateRoleToolsStats();
}

// English note.
function deselectAllRoleTools() {
    document.querySelectorAll('#role-tools-list input[type="checkbox"]').forEach(checkbox => {
        checkbox.checked = false;
        const toolItem = checkbox.closest('.role-tool-item');
        if (toolItem) {
            const toolKey = toolItem.dataset.toolKey;
            const toolName = toolItem.dataset.toolName;
            const isExternal = toolItem.dataset.isExternal === 'true';
            const externalMcp = toolItem.dataset.externalMcp || '';
            if (toolKey) {
                const existingState = roleToolStateMap.get(toolKey);
                roleToolStateMap.set(toolKey, {
                    enabled: false,
                    is_external: isExternal,
                    external_mcp: externalMcp,
                    name: toolName,
                    mcpEnabled: existingState ? existingState.mcpEnabled : true // MCP
                });
            }
        }
    });
    updateRoleToolsStats();
}

// English note.
function searchRoleTools(keyword) {
    roleToolsSearchKeyword = keyword;
    const clearBtn = document.getElementById('role-tools-search-clear');
    if (clearBtn) {
        clearBtn.style.display = keyword ? 'block' : 'none';
    }
    loadRoleTools(1, keyword);
}

// English note.
function clearRoleToolsSearch() {
    document.getElementById('role-tools-search').value = '';
    searchRoleTools('');
}

// English note.
function updateRoleToolsStats() {
    const statsEl = document.getElementById('role-tools-stats');
    if (!statsEl) return;
    
    // English note.
    const currentPageEnabled = Array.from(document.querySelectorAll('#role-tools-list input[type="checkbox"]:checked')).length;
    
    // English note.
    // English note.
    let currentPageEnabledInMCP = 0;
    allRoleTools.forEach(tool => {
        const toolKey = getToolKey(tool);
        const state = roleToolStateMap.get(toolKey);
        // English note.
        const mcpEnabled = state ? (state.mcpEnabled !== false) : (tool.enabled !== false);
        if (mcpEnabled) {
            currentPageEnabledInMCP++;
        }
    });
    
    // English note.
    if (roleUsesAllTools) {
        // English note.
        const totalEnabled = totalEnabledToolsInMCP || 0;
        // English note.
        const currentPageTotal = document.querySelectorAll('#role-tools-list input[type="checkbox"]').length;
        // English note.
        const totalTools = roleToolsPagination.total || 0;
        statsEl.innerHTML = `
            <span title="${_t('roleModal.currentPageSelectedTitle')}">✅ ${_t('roleModal.currentPageSelected', { current: currentPageEnabled, total: currentPageTotal })}</span>
            <span title="${_t('roleModal.totalSelectedTitle')}">📊 ${_t('roleModal.totalSelected', { current: totalEnabled, total: totalTools })} <em>${_t('roleModal.usingAllEnabledTools')}</em></span>
        `;
        return;
    }
    
    // English note.
    let totalSelected = 0;
    roleToolStateMap.forEach(state => {
        // English note.
        if (state.enabled && state.mcpEnabled !== false) {
            totalSelected++;
        }
    });
    
    // English note.
    document.querySelectorAll('#role-tools-list input[type="checkbox"]').forEach(checkbox => {
        const toolItem = checkbox.closest('.role-tool-item');
        if (toolItem) {
            const toolKey = toolItem.dataset.toolKey;
            const savedState = roleToolStateMap.get(toolKey);
            if (savedState && savedState.enabled !== checkbox.checked && savedState.mcpEnabled !== false) {
                // English note.
                if (checkbox.checked && !savedState.enabled) {
                    totalSelected++;
                } else if (!checkbox.checked && savedState.enabled) {
                    totalSelected--;
                }
            }
        }
    });
    
    // English note.
    // English note.
    let totalEnabledForRole = totalEnabledToolsInMCP || 0;
    
    // English note.
    if (totalEnabledForRole === 0) {
        roleToolStateMap.forEach(state => {
            // English note.
            if (state.mcpEnabled !== false) { // mcpEnabled  true  undefined（）
                totalEnabledForRole++;
            }
        });
    }
    
    // English note.
    const currentPageTotal = document.querySelectorAll('#role-tools-list input[type="checkbox"]').length;
    // English note.
    const totalTools = roleToolsPagination.total || 0;
    
    statsEl.innerHTML = `
        <span title="${_t('roleModal.currentPageSelectedTitle')}">✅ ${_t('roleModal.currentPageSelected', { current: currentPageEnabled, total: currentPageTotal })}</span>
        <span title="${_t('roleModal.totalSelectedTitle')}">📊 ${_t('roleModal.totalSelected', { current: totalSelected, total: totalTools })}</span>
    `;
}

// English note.
async function getSelectedRoleTools() {
    // English note.
    saveCurrentRolePageToolStates();
    
    // English note.
    // English note.
    // English note.
    
    // English note.
    // English note.
    // English note.
    
    // English note.
    const selectedTools = [];
    roleToolStateMap.forEach((state, toolKey) => {
        // English note.
        if (state.enabled && state.mcpEnabled !== false) {
            selectedTools.push(toolKey);
        }
    });
    
    // English note.
    // English note.
    
    return selectedTools;
}

// English note.
function setSelectedRoleTools(selectedToolKeys) {
    const selectedSet = new Set(selectedToolKeys || []);
    
    // English note.
    roleToolStateMap.forEach((state, toolKey) => {
        state.enabled = selectedSet.has(toolKey);
    });
    
    // English note.
    document.querySelectorAll('#role-tools-list .role-tool-item').forEach(item => {
        const toolKey = item.dataset.toolKey;
        const checkbox = item.querySelector('input[type="checkbox"]');
        if (toolKey && checkbox) {
            checkbox.checked = selectedSet.has(toolKey);
        }
    });
    
    updateRoleToolsStats();
}

// English note.
async function showAddRoleModal() {
    const modal = document.getElementById('role-modal');
    if (!modal) return;

    document.getElementById('role-modal-title').textContent = _t('roleModal.addRole');
    document.getElementById('role-name').value = '';
    document.getElementById('role-name').disabled = false;
    document.getElementById('role-description').value = '';
    document.getElementById('role-icon').value = '';
    document.getElementById('role-user-prompt').value = '';
    document.getElementById('role-enabled').checked = true;

    // English note.
    const toolsSection = document.getElementById('role-tools-section');
    const defaultHint = document.getElementById('role-tools-default-hint');
    const toolsControls = document.querySelector('.role-tools-controls');
    const toolsList = document.getElementById('role-tools-list');
    const formHint = toolsSection ? toolsSection.querySelector('.form-hint') : null;
    
    if (defaultHint) {
        defaultHint.style.display = 'none';
    }
    if (toolsControls) {
        toolsControls.style.display = 'block';
    }
    if (toolsList) {
        toolsList.style.display = 'block';
    }
    if (formHint) {
        formHint.style.display = 'block';
    }

    // English note.
    roleToolStateMap.clear();
    roleConfiguredTools.clear(); // 
    roleUsesAllTools = false; // 
    roleToolsSearchKeyword = '';
    const searchInput = document.getElementById('role-tools-search');
    if (searchInput) {
        searchInput.value = '';
    }
    const clearBtn = document.getElementById('role-tools-search-clear');
    if (clearBtn) {
        clearBtn.style.display = 'none';
    }
    
    // English note.
    if (toolsList) {
        toolsList.innerHTML = '';
    }

    // English note.
    roleSelectedSkills.clear();
    roleSkillsSearchKeyword = '';
    const skillsSearchInput = document.getElementById('role-skills-search');
    if (skillsSearchInput) {
        skillsSearchInput.value = '';
    }
    const skillsClearBtn = document.getElementById('role-skills-search-clear');
    if (skillsClearBtn) {
        skillsClearBtn.style.display = 'none';
    }

    // English note.
    await loadRoleTools(1, '');
    
    // English note.
    if (toolsList) {
        toolsList.style.display = 'block';
    }
    
    // English note.
    updateRoleToolsStats();

    // English note.
    await loadRoleSkills();

    modal.style.display = 'flex';
}

// English note.
async function editRole(roleName) {
    const role = roles.find(r => r.name === roleName);
    if (!role) {
        showNotification(_t('roleModal.roleNotFound'), 'error');
        return;
    }

    const modal = document.getElementById('role-modal');
    if (!modal) return;

    document.getElementById('role-modal-title').textContent = _t('roleModal.editRole');
    document.getElementById('role-name').value = role.name;
    document.getElementById('role-name').disabled = true; // 
    document.getElementById('role-description').value = role.description || '';
    // English note.
    let iconValue = role.icon || '';
    if (iconValue && iconValue.startsWith('\\U')) {
        // English note.
        try {
            const codePoint = parseInt(iconValue.substring(2), 16);
            iconValue = String.fromCodePoint(codePoint);
        } catch (e) {
            // English note.
        }
    }
    document.getElementById('role-icon').value = iconValue;
    document.getElementById('role-user-prompt').value = role.user_prompt || '';
    document.getElementById('role-enabled').checked = role.enabled !== false;

    // English note.
    const isDefaultRole = roleName === '';
    const toolsSection = document.getElementById('role-tools-section');
    const defaultHint = document.getElementById('role-tools-default-hint');
    const toolsControls = document.querySelector('.role-tools-controls');
    const toolsList = document.getElementById('role-tools-list');
    const formHint = toolsSection ? toolsSection.querySelector('.form-hint') : null;
    
    if (isDefaultRole) {
        // English note.
        if (defaultHint) {
            defaultHint.style.display = 'block';
        }
        if (toolsControls) {
            toolsControls.style.display = 'none';
        }
        if (toolsList) {
            toolsList.style.display = 'none';
        }
        if (formHint) {
            formHint.style.display = 'none';
        }
    } else {
        // English note.
        if (defaultHint) {
            defaultHint.style.display = 'none';
        }
        if (toolsControls) {
            toolsControls.style.display = 'block';
        }
        if (toolsList) {
            toolsList.style.display = 'block';
        }
        if (formHint) {
            formHint.style.display = 'block';
        }

        // English note.
        roleToolStateMap.clear();
        roleConfiguredTools.clear(); // 
        roleToolsSearchKeyword = '';
        const searchInput = document.getElementById('role-tools-search');
        if (searchInput) {
            searchInput.value = '';
        }
        const clearBtn = document.getElementById('role-tools-search-clear');
        if (clearBtn) {
            clearBtn.style.display = 'none';
        }

        // English note.
        const selectedTools = role.tools || (role.mcps && role.mcps.length > 0 ? role.mcps : []);
        
        // English note.
        roleUsesAllTools = !role.tools || role.tools.length === 0;
        
        // English note.
        if (selectedTools.length > 0) {
            selectedTools.forEach(toolKey => {
                roleConfiguredTools.add(toolKey);
            });
        }
        
        // English note.
        if (selectedTools.length > 0) {
            roleUsesAllTools = false; // ，
            // English note.
            selectedTools.forEach(toolKey => {
                // English note.
                if (!roleToolStateMap.has(toolKey)) {
                    roleToolStateMap.set(toolKey, {
                        enabled: true,
                        is_external: false,
                        external_mcp: '',
                        name: toolKey.split('::').pop() || toolKey // toolKey
                    });
                } else {
                    // English note.
                    const state = roleToolStateMap.get(toolKey);
                    state.enabled = true;
                }
            });
        }

        // English note.
        await loadRoleTools(1, '');
        
        // English note.
        if (roleUsesAllTools) {
            // English note.
            document.querySelectorAll('#role-tools-list input[type="checkbox"]').forEach(checkbox => {
                const toolItem = checkbox.closest('.role-tool-item');
                if (toolItem) {
                    const toolKey = toolItem.dataset.toolKey;
                    const toolName = toolItem.dataset.toolName;
                    const isExternal = toolItem.dataset.isExternal === 'true';
                    const externalMcp = toolItem.dataset.externalMcp || '';
                    if (toolKey) {
                        const state = roleToolStateMap.get(toolKey);
                        // English note.
                        // English note.
                        const shouldEnable = state ? (state.mcpEnabled !== false) : true;
                        checkbox.checked = shouldEnable;
                        if (state) {
                            state.enabled = shouldEnable;
                        } else {
                            // English note.
                            roleToolStateMap.set(toolKey, {
                                enabled: shouldEnable,
                                is_external: isExternal,
                                external_mcp: externalMcp,
                                name: toolName,
                                mcpEnabled: true // ，loadRoleTools
                            });
                        }
                    }
                }
            });
            // English note.
            updateRoleToolsStats();
        } else if (selectedTools.length > 0) {
            // English note.
            setSelectedRoleTools(selectedTools);
        }
    }

    // English note.
    await loadRoleSkills();
    // English note.
    const selectedSkills = role.skills || [];
    roleSelectedSkills.clear();
    selectedSkills.forEach(skill => {
        roleSelectedSkills.add(skill);
    });
    renderRoleSkills();

    modal.style.display = 'flex';
}

// English note.
function closeRoleModal() {
    const modal = document.getElementById('role-modal');
    if (modal) {
        modal.style.display = 'none';
    }
}

// English note.
function getAllSelectedRoleTools() {
    // English note.
    saveCurrentRolePageToolStates();
    
    // English note.
    const selectedTools = [];
    roleToolStateMap.forEach((state, toolKey) => {
        if (state.enabled) {
            selectedTools.push({
                key: toolKey,
                name: state.name || toolKey.split('::').pop() || toolKey,
                mcpEnabled: state.mcpEnabled !== false // mcpEnabled  false ，
            });
        }
    });
    
    return selectedTools;
}

// English note.
function getDisabledTools(selectedTools) {
    return selectedTools.filter(tool => {
        const state = roleToolStateMap.get(tool.key);
        // English note.
        return state && state.mcpEnabled === false;
    });
}

// English note.
async function loadAllToolsToStateMap() {
    try {
        const pageSize = 100; // 
        let page = 1;
        let hasMore = true;
        
        // English note.
        while (hasMore) {
            const url = `/api/config/tools?page=${page}&page_size=${pageSize}`;
            const response = await apiFetch(url);
            if (!response.ok) {
                throw new Error('');
            }
            
            const result = await response.json();
            
            // English note.
            result.tools.forEach(tool => {
                const toolKey = getToolKey(tool);
                if (!roleToolStateMap.has(toolKey)) {
                    // English note.
                    let enabled = false;
                    if (roleUsesAllTools) {
                        // English note.
                        enabled = tool.enabled ? true : false;
                    } else {
                        // English note.
                        enabled = roleConfiguredTools.has(toolKey);
                    }
                    roleToolStateMap.set(toolKey, {
                        enabled: enabled,
                        is_external: tool.is_external || false,
                        external_mcp: tool.external_mcp || '',
                        name: tool.name,
                        mcpEnabled: tool.enabled // MCP
                    });
                } else {
                    // English note.
                    const state = roleToolStateMap.get(toolKey);
                    state.is_external = tool.is_external || false;
                    state.external_mcp = tool.external_mcp || '';
                    state.mcpEnabled = tool.enabled; // MCP
                    if (!state.name || state.name === toolKey.split('::').pop()) {
                        state.name = tool.name; // 
                    }
                }
            });
            
            // English note.
            if (page >= result.total_pages) {
                hasMore = false;
            } else {
                page++;
            }
        }
    } catch (error) {
        console.error(':', error);
        throw error;
    }
}

// English note.
async function saveRole() {
    const name = document.getElementById('role-name').value.trim();
    if (!name) {
        showNotification(_t('roleModal.roleNameRequired'), 'error');
        return;
    }

    const description = document.getElementById('role-description').value.trim();
    let icon = document.getElementById('role-icon').value.trim();
    // English note.
    if (icon) {
        // English note.
        const codePoint = icon.codePointAt(0);
        if (codePoint && codePoint > 0x7F) {
            // English note.
            icon = '\\U' + codePoint.toString(16).toUpperCase().padStart(8, '0');
        }
    }
    const userPrompt = document.getElementById('role-user-prompt').value.trim();
    const enabled = document.getElementById('role-enabled').checked;

    const isEdit = document.getElementById('role-name').disabled;
    
    // English note.
    const isDefaultRole = name === '';
    
    // English note.
    const isFirstUserRole = !isEdit && !isDefaultRole && roles.filter(r => r.name !== '').length === 0;
    
    // English note.
    // English note.
    let tools = [];
    let disabledTools = []; // MCP
    
    if (!isDefaultRole) {
        // English note.
        saveCurrentRolePageToolStates();
        
        // English note.
        let allSelectedTools = getAllSelectedRoleTools();
        
        // English note.
        if (isFirstUserRole && allSelectedTools.length === 0) {
            roleUsesAllTools = true;
            showNotification(_t('roleModal.firstRoleNoToolsHint'), 'info');
        } else if (roleUsesAllTools) {
            // English note.
            // English note.
            let hasUnselectedTools = false;
            roleToolStateMap.forEach((state) => {
                // English note.
                if (state.mcpEnabled !== false && !state.enabled) {
                    hasUnselectedTools = true;
                }
            });
            
            // English note.
            if (hasUnselectedTools) {
                // English note.
                // English note.
                await loadAllToolsToStateMap();
                
                // English note.
                // English note.
                roleToolStateMap.forEach((state, toolKey) => {
                    // English note.
                    // English note.
                    if (state.mcpEnabled !== false && state.enabled !== false) {
                        state.enabled = true;
                    }
                });
                
                roleUsesAllTools = false;
            } else {
                // English note.
                // English note.
                await loadAllToolsToStateMap();
                
                // English note.
                let hasDisabledToolsSelected = false;
                roleToolStateMap.forEach((state) => {
                    if (state.enabled && state.mcpEnabled === false) {
                        hasDisabledToolsSelected = true;
                    }
                });
                
                // English note.
                if (!hasDisabledToolsSelected) {
                    roleToolStateMap.forEach((state) => {
                        if (state.mcpEnabled !== false) {
                            state.enabled = true;
                        }
                    });
                }
                
                // English note.
                allSelectedTools = getAllSelectedRoleTools();
            }
        }
        
        // English note.
        disabledTools = getDisabledTools(allSelectedTools);
        
        // English note.
        if (disabledTools.length > 0) {
            const toolNames = disabledTools.map(t => t.name).join('、');
            const message = ` ${disabledTools.length} MCP，：\n\n${toolNames}\n\n"MCP"，。\n\n？（）`;
            
            if (!confirm(message)) {
                return; // 
            }
        }
        
        // English note.
        if (!roleUsesAllTools) {
            // English note.
            tools = await getSelectedRoleTools();
        }
    }

    // English note.
    const skills = Array.from(roleSelectedSkills);

    const roleData = {
        name: name,
        description: description,
        icon: icon || undefined, // ，
        user_prompt: userPrompt,
        tools: tools, // ，
        skills: skills, // Skills
        enabled: enabled
    };
    const url = isEdit ? `/api/roles/${encodeURIComponent(name)}` : '/api/roles';
    const method = isEdit ? 'PUT' : 'POST';

    try {
        const response = await apiFetch(url, {
            method: method,
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(roleData)
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || '');
        }

        // English note.
        if (disabledTools.length > 0) {
            let toolNames = disabledTools.map(t => t.name).join('、');
            // English note.
            if (toolNames.length > 100) {
                toolNames = toolNames.substring(0, 100) + '...';
            }
            showNotification(
                `${isEdit ? '' : ''}， ${disabledTools.length} MCP：${toolNames}。"MCP"，。`,
                'warning'
            );
        } else {
            showNotification(isEdit ? '' : '', 'success');
        }
        
        closeRoleModal();
        await refreshRoles();
    } catch (error) {
        console.error(':', error);
        showNotification(': ' + error.message, 'error');
    }
}

// English note.
async function deleteRole(roleName) {
    if (roleName === '') {
        showNotification(_t('roleModal.cannotDeleteDefaultRole'), 'error');
        return;
    }

    if (!confirm(`"${roleName}"？。`)) {
        return;
    }

    try {
        const response = await apiFetch(`/api/roles/${encodeURIComponent(roleName)}`, {
            method: 'DELETE'
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || '');
        }

        showNotification('', 'success');
        
        // English note.
        if (currentRole === roleName) {
            handleRoleChange('');
        }

        await refreshRoles();
    } catch (error) {
        console.error(':', error);
        showNotification(': ' + error.message, 'error');
    }
}

// English note.
if (typeof switchPage === 'function') {
    const originalSwitchPage = switchPage;
    switchPage = function(page) {
        originalSwitchPage(page);
        if (page === 'roles-management') {
            loadRoles().then(() => renderRolesList());
        }
    };
}

// English note.
document.addEventListener('click', (e) => {
    const roleSelectModal = document.getElementById('role-select-modal');
    if (roleSelectModal && e.target === roleSelectModal) {
        closeRoleSelectModal();
    }

    const roleModal = document.getElementById('role-modal');
    if (roleModal && e.target === roleModal) {
        closeRoleModal();
    }

    // English note.
    const roleSelectionPanel = document.getElementById('role-selection-panel');
    const roleSelectorWrapper = document.querySelector('.role-selector-wrapper');
    if (roleSelectionPanel && roleSelectionPanel.style.display !== 'none' && roleSelectionPanel.style.display) {
        // English note.
        if (!roleSelectorWrapper?.contains(e.target)) {
            closeRoleSelectionPanel();
        }
    }
});

// English note.
document.addEventListener('DOMContentLoaded', () => {
    loadRoles();
    updateRoleSelectorDisplay();
});

// English note.
document.addEventListener('languagechange', () => {
    updateRoleSelectorDisplay();
});

// English note.
function getCurrentRole() {
    return currentRole || '';
}

// English note.
if (typeof window !== 'undefined') {
    window.getCurrentRole = getCurrentRole;
    window.toggleRoleSelectionPanel = toggleRoleSelectionPanel;
    window.closeRoleSelectionPanel = closeRoleSelectionPanel;
    window.currentSelectedRole = getCurrentRole();
    
    // English note.
    const originalHandleRoleChange = handleRoleChange;
    handleRoleChange = function(roleName) {
        originalHandleRoleChange(roleName);
        if (typeof window !== 'undefined') {
            window.currentSelectedRole = getCurrentRole();
        }
    };
}

// English note.

// English note.
async function loadRoleSkills() {
    try {
        const response = await apiFetch('/api/roles/skills/list');
        if (!response.ok) {
            throw new Error('skills');
        }
        const data = await response.json();
        allRoleSkills = data.skills || [];
        renderRoleSkills();
    } catch (error) {
        console.error('skills:', error);
        allRoleSkills = [];
        const skillsList = document.getElementById('role-skills-list');
        if (skillsList) {
            skillsList.innerHTML = '<div class="skills-error">' + _t('roleModal.loadSkillsFailed') + ': ' + error.message + '</div>';
        }
    }
}

// English note.
function renderRoleSkills() {
    const skillsList = document.getElementById('role-skills-list');
    if (!skillsList) return;

    // English note.
    let filteredSkills = allRoleSkills;
    if (roleSkillsSearchKeyword) {
        const keyword = roleSkillsSearchKeyword.toLowerCase();
        filteredSkills = allRoleSkills.filter(skill => 
            skill.toLowerCase().includes(keyword)
        );
    }

    if (filteredSkills.length === 0) {
        skillsList.innerHTML = '<div class="skills-empty">' + 
            (roleSkillsSearchKeyword ? _t('roleModal.noMatchingSkills') : _t('roleModal.noSkillsAvailable')) + 
            '</div>';
        updateRoleSkillsStats();
        return;
    }

    // English note.
    skillsList.innerHTML = filteredSkills.map(skill => {
        const isSelected = roleSelectedSkills.has(skill);
        return `
            <div class="role-skill-item" data-skill="${skill}">
                <label class="checkbox-label">
                    <input type="checkbox" class="modern-checkbox" 
                           ${isSelected ? 'checked' : ''} 
                           onchange="toggleRoleSkill('${skill}', this.checked)" />
                    <span class="checkbox-custom"></span>
                    <span class="checkbox-text">${escapeHtml(skill)}</span>
                </label>
            </div>
        `;
    }).join('');

    updateRoleSkillsStats();
}

// English note.
function toggleRoleSkill(skill, checked) {
    if (checked) {
        roleSelectedSkills.add(skill);
    } else {
        roleSelectedSkills.delete(skill);
    }
    updateRoleSkillsStats();
}

// English note.
function selectAllRoleSkills() {
    let filteredSkills = allRoleSkills;
    if (roleSkillsSearchKeyword) {
        const keyword = roleSkillsSearchKeyword.toLowerCase();
        filteredSkills = allRoleSkills.filter(skill => 
            skill.toLowerCase().includes(keyword)
        );
    }
    filteredSkills.forEach(skill => {
        roleSelectedSkills.add(skill);
    });
    renderRoleSkills();
}

// English note.
function deselectAllRoleSkills() {
    let filteredSkills = allRoleSkills;
    if (roleSkillsSearchKeyword) {
        const keyword = roleSkillsSearchKeyword.toLowerCase();
        filteredSkills = allRoleSkills.filter(skill => 
            skill.toLowerCase().includes(keyword)
        );
    }
    filteredSkills.forEach(skill => {
        roleSelectedSkills.delete(skill);
    });
    renderRoleSkills();
}

// English note.
function searchRoleSkills(keyword) {
    roleSkillsSearchKeyword = keyword;
    const clearBtn = document.getElementById('role-skills-search-clear');
    if (clearBtn) {
        clearBtn.style.display = keyword ? 'block' : 'none';
    }
    renderRoleSkills();
}

// English note.
function clearRoleSkillsSearch() {
    const searchInput = document.getElementById('role-skills-search');
    if (searchInput) {
        searchInput.value = '';
    }
    roleSkillsSearchKeyword = '';
    const clearBtn = document.getElementById('role-skills-search-clear');
    if (clearBtn) {
        clearBtn.style.display = 'none';
    }
    renderRoleSkills();
}

// English note.
function updateRoleSkillsStats() {
    const statsEl = document.getElementById('role-skills-stats');
    if (!statsEl) return;

    let filteredSkills = allRoleSkills;
    if (roleSkillsSearchKeyword) {
        const keyword = roleSkillsSearchKeyword.toLowerCase();
        filteredSkills = allRoleSkills.filter(skill => 
            skill.toLowerCase().includes(keyword)
        );
    }

    const selectedCount = Array.from(roleSelectedSkills).filter(skill => 
        filteredSkills.includes(skill)
    ).length;

    statsEl.textContent = _t('roleModal.skillsSelectedCount', { count: selectedCount, total: filteredSkills.length });
}

// English note.
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
