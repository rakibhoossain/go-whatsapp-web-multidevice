export default {
    name: 'CampaignGroups',
    data() {
        return {
            loading: false,
            groups: [],
            customers: [],
            selectedGroup: null,
            form: {
                name: '',
                description: ''
            },
            editingId: null,
            total: 0,
            searchQuery: '',
            searchTimeout: null,
            customersPage: 1,
            customersLoading: false,
            customersHasMore: true,
            filterMode: 'all', // all, member, non_member
            bulkSelectedIds: [],
            bulkLoading: false
        }
    },
    computed: {
        totalPages() {
            return Math.ceil(this.total / this.pageSize);
        },
        isAllSelected() {
            return this.customers.length > 0 && this.customers.every(c => this.bulkSelectedIds.includes(c.id));
        }
    },
    methods: {
        async openModal() {
            $('#modalCampaignGroups').modal('show');
            await this.loadGroups();
        },
        async loadGroups() {
            try {
                this.loading = true;
                const response = await window.http.get(`/campaign/groups?page=${this.page}&page_size=${this.pageSize}`);
                this.groups = response.data.results.groups || [];
                this.total = response.data.results.total || 0;
            } catch (error) {
                showErrorInfo(error.response?.data?.message || error.message);
            } finally {
                this.loading = false;
            }
        },
        async loadCustomers(reset = false) {
            if (this.customersLoading) return;

            if (reset) {
                this.customersPage = 1;
                this.customers = [];
                this.customersHasMore = true;
            }

            if (!this.customersHasMore) return;

            try {
                this.customersLoading = true;
                let url = `/campaign/customers?page=${this.customersPage}&page_size=20`;
                if (this.searchQuery) {
                    url += `&search=${encodeURIComponent(this.searchQuery)}`;
                }
                if (this.filterMode !== 'all' && this.selectedGroup) {
                    url += `&filter_group_id=${this.selectedGroup.id}&filter_type=${this.filterMode}`;
                }
                const response = await window.http.get(url);
                const newCustomers = response.data.results.customers || [];

                // Transform customers for readiness if needed (though API sends is_ready)
                // We rely on API's is_ready which checks phone_valid && whatsapp_exists

                if (newCustomers.length < 20) {
                    this.customersHasMore = false;
                }

                if (reset) {
                    this.customers = newCustomers;
                } else {
                    this.customers = [...this.customers, ...newCustomers];
                }
                this.customersPage++;
            } catch (error) {
                console.error('Failed to load customers:', error);
            } finally {
                this.customersLoading = false;
            }
        },
        handleScroll(e) {
            const { scrollTop, scrollHeight, clientHeight } = e.target;
            if (scrollTop + clientHeight >= scrollHeight - 50) {
                this.loadCustomers();
            }
        },
        async changeFilter() {
            this.customersPage = 1;
            await this.loadCustomers(true);
        },
        toggleBulkSelect(customerId) {
            const index = this.bulkSelectedIds.indexOf(customerId);
            if (index > -1) {
                this.bulkSelectedIds.splice(index, 1);
            } else {
                this.bulkSelectedIds.push(customerId);
            }
        },
        toggleSelectAll() {
            if (this.isAllSelected) {
                this.bulkSelectedIds = [];
            } else {
                this.bulkSelectedIds = this.customers.map(c => c.id);
            }
        },
        async bulkAdd() {
            if (this.bulkSelectedIds.length === 0 || !this.selectedGroup) return;

            // Filter only valid customers
            const validIds = this.bulkSelectedIds.filter(id => {
                const customer = this.customers.find(c => c.id === id);
                return customer && customer.is_ready;
            });

            if (validIds.length === 0) {
                showErrorInfo('No valid customers selected (must be phone valid & on WhatsApp)');
                return;
            }

            if (validIds.length < this.bulkSelectedIds.length) {
                if (!confirm(`Only ${validIds.length} of ${this.bulkSelectedIds.length} selected customers are valid. Proceed?`)) return;
            }

            try {
                this.bulkLoading = true;
                await window.http.post(`/campaign/groups/${this.selectedGroup.id}/members`, {
                    customer_ids: validIds
                });
                showSuccessInfo('Customers added to group');
                this.bulkSelectedIds = [];

                // Refresh data
                await this.refreshGroupData();
                if (this.filterMode === 'non_member') {
                    await this.loadCustomers(true);
                }
            } catch (error) {
                showErrorInfo(error.response?.data?.message || error.message);
            } finally {
                this.bulkLoading = false;
            }
        },
        async bulkRemove() {
            if (this.bulkSelectedIds.length === 0 || !this.selectedGroup) return;
            if (!confirm(`Remove ${this.bulkSelectedIds.length} customers from group?`)) return;

            try {
                this.bulkLoading = true;
                // Since we don't have bulk delete API, we loop. 
                // In production with large lists, a bulk delete endpoint is better.
                const promises = this.bulkSelectedIds.map(id =>
                    window.http.delete(`/campaign/groups/${this.selectedGroup.id}/members/${id}`)
                );
                await Promise.all(promises);

                showSuccessInfo('Customers removed from group');
                this.bulkSelectedIds = [];

                // Refresh data
                await this.refreshGroupData();
                if (this.filterMode === 'member') {
                    await this.loadCustomers(true);
                }
            } catch (error) {
                showErrorInfo(error.response?.data?.message || error.message);
            } finally {
                this.bulkLoading = false;
            }
        },
        triggerCSVImport() {
            this.$refs.csvInput.click();
        },
        async handleCSVUpload(event) {
            const file = event.target.files[0];
            if (!file) return;

            // Reset input so same file can be selected again if needed
            event.target.value = '';

            if (!confirm(`Import customers from ${file.name} to this group?`)) return;

            const formData = new FormData();
            formData.append('file', file);
            formData.append('group_id', this.selectedGroup.id);

            try {
                this.bulkLoading = true;
                const response = await window.http.post('/campaign/customers/import', formData, {
                    headers: { 'Content-Type': 'multipart/form-data' }
                });

                const result = response.data.results;
                showSuccessInfo(`Imported: ${result.imported}, Errors: ${result.errors?.length || 0}`);

                if (result.errors && result.errors.length > 0) {
                    console.warn('Import errors:', result.errors);
                    showErrorInfo(`Completed with errors. Check console.`);
                }

                // Reload list
                await this.loadCustomers(true);
                await this.refreshGroupData();
            } catch (error) {
                showErrorInfo(error.response?.data?.message || error.message);
            } finally {
                this.bulkLoading = false;
            }
        },
        async refreshGroupData() {
            const response = await window.http.get(`/campaign/groups/${this.selectedGroup.id}`);
            const groupData = response.data.results;
            this.selectedCustomerIds = (groupData.customers || []).map(c => c.id);

            // Update group list count
            const group = this.groups.find(g => g.id === this.selectedGroup.id);
            if (group) {
                group.customer_count = this.selectedCustomerIds.length;
            }
        },
        handleSearch() {
            clearTimeout(this.searchTimeout);
            this.searchTimeout = setTimeout(() => {
                this.loadCustomers(true);
            }, 500);
        },
        openCreateModal() {
            this.resetForm();
            this.editingId = null;
            $('#modalCampaignGroupForm').modal('show');
        },
        openEditModal(group) {
            this.form = {
                name: group.name,
                description: group.description || ''
            };
            this.editingId = group.id;
            $('#modalCampaignGroupForm').modal('show');
        },
        resetForm() {
            this.form = { name: '', description: '' };
        },
        async submitForm() {
            if (!this.form.name.trim()) {
                showErrorInfo('Group name is required');
                return;
            }
            try {
                this.loading = true;
                const payload = {
                    name: this.form.name,
                    description: this.form.description || null
                };

                if (this.editingId) {
                    await window.http.put(`/campaign/groups/${this.editingId}`, payload);
                    showSuccessInfo('Group updated');
                } else {
                    await window.http.post('/campaign/groups', payload);
                    showSuccessInfo('Group created');
                }
                $('#modalCampaignGroupForm').modal('hide');
                await this.loadGroups();
            } catch (error) {
                showErrorInfo(error.response?.data?.message || error.message);
            } finally {
                this.loading = false;
            }
        },
        async deleteGroup(id) {
            if (!confirm('Are you sure you want to delete this group?')) return;
            try {
                await window.http.delete(`/campaign/groups/${id}`);
                showSuccessInfo('Group deleted');
                await this.loadGroups();
            } catch (error) {
                showErrorInfo(error.response?.data?.message || error.message);
            }
        },
        async openMembersModal(group) {
            this.selectedGroup = group;
            this.searchQuery = ''; // Reset search on open
            this.filterMode = 'all';
            this.bulkSelectedIds = [];
            this.customers = []; // Clear previous list
            this.customersLoading = false; // Reset loading state just in case
            this.customersPage = 1;
            this.customersHasMore = true;

            $('#modalCampaignGroupMembers').modal({
                detachable: true, // Revert to true for layout fix
                observeChanges: true,
                onHidden: () => {
                    this.customers = [];
                }
            }).modal('show');

            // Load after show to visual flicker or before? Before is better for UX, but if it fails silently...
            // Let's await it.
            await this.loadCustomers(true);

            const response = await window.http.get(`/campaign/groups/${group.id}`);
            const groupData = response.data.results;
            this.selectedCustomerIds = (groupData.customers || []).map(c => c.id);
        },
        isCustomerInGroup(customerId) {
            return this.selectedCustomerIds.includes(customerId);
        },
        async toggleCustomer(customer) {
            if (!this.selectedGroup) return;

            const isMember = this.selectedCustomerIds.includes(customer.id);

            if (!isMember && !customer.is_ready) {
                showErrorInfo('Cannot add invalid customer (must be phone valid & on WhatsApp)');
                return;
            }

            // Optimistic update
            if (isMember) {
                const index = this.selectedCustomerIds.indexOf(customer.id);
                this.selectedCustomerIds.splice(index, 1);
            } else {
                this.selectedCustomerIds.push(customer.id);
            }

            try {
                if (isMember) {
                    // Remove member
                    await window.http.delete(`/campaign/groups/${this.selectedGroup.id}/members/${customer.id}`);
                } else {
                    // Add member
                    await window.http.post(`/campaign/groups/${this.selectedGroup.id}/members`, {
                        customer_ids: [customer.id]
                    });
                }

                // Refresh IDs to ensure consistency
                const response = await window.http.get(`/campaign/groups/${this.selectedGroup.id}`);
                const groupData = response.data.results;
                this.selectedCustomerIds = (groupData.customers || []).map(c => c.id);

                // Update group count in list
                const group = this.groups.find(g => g.id === this.selectedGroup.id);
                if (group) {
                    group.customer_count = this.selectedCustomerIds.length;
                }

                // If filtering, reload because item status changed
                if (this.filterMode !== 'all') {
                    this.loadCustomers(true);
                }
            } catch (error) {
                // Revert update on error
                if (isMember) {
                    this.selectedCustomerIds.push(customer.id);
                } else {
                    const index = this.selectedCustomerIds.indexOf(customer.id);
                    this.selectedCustomerIds.splice(index, 1);
                }
                showErrorInfo(error.response?.data?.message || error.message);
            }
        },
        getStatusColor(status) {
            return {
                'pending': 'grey',
                'valid': 'green',
                'invalid': 'red'
            }[status] || 'grey';
        },
        nextPage() {
            if (this.page < this.totalPages) {
                this.page++;
                this.loadGroups();
            }
        },
        prevPage() {
            if (this.page > 1) {
                this.page--;
                this.loadGroups();
            }
        }
    },
    template: `
    <div class="purple card" @click="openModal" style="cursor: pointer">
        <div class="content">
            <a class="ui purple right ribbon label">Campaign</a>
            <div class="header">Customer Groups</div>
            <div class="description">
                Organize customers into groups
            </div>
        </div>
    </div>
    
    <!-- Groups List Modal -->
    <div class="ui modal" id="modalCampaignGroups">
        <i class="close icon"></i>
        <div class="header">
            <i class="object group icon"></i> Customer Groups
            <button class="ui green right floated button" @click.stop="openCreateModal">
                <i class="plus icon"></i> New Group
            </button>
        </div>
        <div class="scrolling content">
            <div class="ui active inverted dimmer" v-if="loading">
                <div class="ui loader"></div>
            </div>
            <div class="ui relaxed divided list">
                <div class="item" v-for="group in groups" :key="group.id" style="padding: 15px 0">
                    <div class="right floated content">
                        <button class="ui mini blue button" @click.stop="openMembersModal(group)">
                            <i class="users icon"></i> Members ({{ group.customer_count || 0 }})
                        </button>
                        <button class="ui mini yellow button" @click.stop="openEditModal(group)">
                            <i class="edit icon"></i>
                        </button>
                        <button class="ui mini red button" @click.stop="deleteGroup(group.id)">
                            <i class="trash icon"></i>
                        </button>
                    </div>
                    <i class="large folder middle aligned icon"></i>
                    <div class="content">
                        <div class="header">{{ group.name }}</div>
                        <div class="description">{{ group.description || 'No description' }}</div>
                    </div>
                </div>
            </div>
            <div class="ui message" v-if="groups.length === 0 && !loading">
                No groups created yet. Create a group to organize your customers.
            </div>
            
            <!-- Pagination -->
            <div class="ui pagination menu" v-if="totalPages > 1" style="display: flex; justify-content: center; margin-top: 20px;">
                <a class="icon item" @click="prevPage" :class="{ disabled: page === 1 }">
                    <i class="left chevron icon"></i>
                </a>
                <div class="item">
                    Page {{ page }} of {{ totalPages }}
                </div>
                <a class="icon item" @click="nextPage" :class="{ disabled: page === totalPages }">
                    <i class="right chevron icon"></i>
                </a>
            </div>
        </div>
    </div>
    
    <!-- Group Form Modal -->
    <div class="ui small modal" id="modalCampaignGroupForm">
        <i class="close icon"></i>
        <div class="header">{{ editingId ? 'Edit Group' : 'Create Group' }}</div>
        <div class="content">
            <form class="ui form">
                <div class="required field">
                    <label>Group Name</label>
                    <input v-model="form.name" type="text" placeholder="VIP Customers">
                </div>
                <div class="field">
                    <label>Description</label>
                    <textarea v-model="form.description" placeholder="Group description..."></textarea>
                </div>
            </form>
        </div>
        <div class="actions">
            <button class="ui positive button" :class="{loading: loading}" @click="submitForm">
                <i class="check icon"></i> Save
            </button>
        </div>
    </div>
    
    <!-- Group Members Modal -->
    <div class="ui modal" id="modalCampaignGroupMembers">
        <i class="close icon"></i>
        <div class="header">
            <i class="users icon"></i> Manage Members - {{ selectedGroup?.name }}
            <button class="ui mini teal right floated button" @click="triggerCSVImport" :class="{loading: bulkLoading}">
                <i class="upload icon"></i> Import CSV
            </button>
            <input type="file" ref="csvInput" style="display: none" accept=".csv" @change="handleCSVUpload">
        </div>
        <div class="scrolling content">
            <div class="ui info message">
                <p>Select customers to add to this group:</p>
            </div>
            
            <div class="ui grid" style="margin-bottom: 10px">
                <div class="eight wide column">
                    <div class="ui fluid icon input">
                        <input type="text" v-model="searchQuery" @input="handleSearch" placeholder="Search customers...">
                        <i class="search icon"></i>
                    </div>
                </div>
                <div class="eight wide column">
                    <select class="ui fluid dropdown" v-model="filterMode" @change="changeFilter">
                        <option value="all">All Customers</option>
                        <option value="member">Members Only</option>
                        <option value="non_member">Non-Members Only</option>
                    </select>
                </div>
            </div>

            <div class="ui segment" v-if="bulkSelectedIds.length > 0">
                <div class="ui grid middle aligned">
                    <div class="eight wide column">
                        <strong>{{ bulkSelectedIds.length }} customers selected</strong>
                    </div>
                    <div class="eight wide column right aligned">
                        <button class="ui mini blue button" :class="{loading: bulkLoading}" @click="bulkAdd">
                            Add Selected
                        </button>
                        <button class="ui mini red button" :class="{loading: bulkLoading}" @click="bulkRemove">
                            Remove Selected
                        </button>
                    </div>
                </div>
            </div>

            <div class="ui middle aligned divided selection list" style="max-height: 400px; overflow-y: auto" @scroll="handleScroll">
                <div class="item" style="background: #f9f9f9; padding: 10px !important;">
                    <div class="ui checkbox">
                        <input type="checkbox" :checked="isAllSelected" @click.stop="toggleSelectAll">
                        <label>Select All Loaded</label>
                    </div>
                </div>
                <div class="item" v-for="customer in customers" :key="customer.id" 
                     @click="toggleCustomer(customer)" style="cursor: pointer" :class="{disabled: !customer.is_ready && !isCustomerInGroup(customer.id)}">
                    <div class="left floated content" style="margin-right: 10px;">
                        <div class="ui checkbox" @click.stop :class="{disabled: !customer.is_ready}">
                            <input type="checkbox" :checked="bulkSelectedIds.includes(customer.id)" @change="toggleBulkSelect(customer.id)" :disabled="!customer.is_ready">
                            <label></label>
                        </div>
                    </div>
                    <div class="right floated content">
                        <div class="ui toggle checkbox" :class="{disabled: !customer.is_ready && !isCustomerInGroup(customer.id)}">
                            <input type="checkbox" :checked="isCustomerInGroup(customer.id)" @click.stop="toggleCustomer(customer)" :disabled="!customer.is_ready && !isCustomerInGroup(customer.id)">
                            <label></label>
                        </div>
                    </div>
                    <i class="large user circle icon"></i>
                    <div class="content">
                        <div class="header">
                            {{ customer.full_name || customer.phone }}
                            <div class="ui horizontal labels" style="margin-left: 5px">
                                <span v-if="customer.is_ready" class="ui mini green label">Ready</span>
                                <span v-else class="ui mini red label">Invalid</span>
                            </div>
                        </div>
                        <div class="description">
                            {{ customer.phone }} 
                            <span v-if="!customer.is_ready" style="font-size: 0.8em; color: #db2828;">
                                (Phone: {{ customer.phone_valid }}, WA: {{ customer.whatsapp_exists }})
                            </span>
                        </div>
                    </div>
                </div>
            </div>
            <div class="ui message" v-if="customers.length === 0">
                No customers available. Add customers first.
            </div>
        </div>
    </div>
    `
}
