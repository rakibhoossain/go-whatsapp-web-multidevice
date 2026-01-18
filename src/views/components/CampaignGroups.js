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
            selectedCustomerIds: [],
            page: 1,
            pageSize: 10,
            total: 0,
            searchQuery: '',
            searchTimeout: null
        }
    },
    computed: {
        totalPages() {
            return Math.ceil(this.total / this.pageSize);
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
        async loadCustomers() {
            try {
                let url = '/campaign/customers?page_size=1000';
                if (this.searchQuery) {
                    url += `&search=${encodeURIComponent(this.searchQuery)}`;
                }
                const response = await window.http.get(url);
                this.customers = response.data.results.customers || [];
            } catch (error) {
                console.error('Failed to load customers:', error);
            }
        },
        handleSearch() {
            clearTimeout(this.searchTimeout);
            this.searchTimeout = setTimeout(() => {
                this.loadCustomers();
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
            await this.loadCustomers();
            const response = await window.http.get(`/campaign/groups/${group.id}`);
            const groupData = response.data.results;
            this.selectedCustomerIds = (groupData.customers || []).map(c => c.id);
            $('#modalCampaignGroupMembers').modal('show');
        },
        isCustomerInGroup(customerId) {
            return this.selectedCustomerIds.includes(customerId);
        },
        toggleCustomer(customerId) {
            const index = this.selectedCustomerIds.indexOf(customerId);
            if (index > -1) {
                this.selectedCustomerIds.splice(index, 1);
            } else {
                this.selectedCustomerIds.push(customerId);
            }
        },
        async saveMembers() {
            if (!this.selectedGroup) return;
            try {
                this.loading = true;
                await window.http.post(`/campaign/groups/${this.selectedGroup.id}/members`, {
                    customer_ids: this.selectedCustomerIds
                });
                showSuccessInfo('Group members updated');
                $('#modalCampaignGroupMembers').modal('hide');
                await this.loadGroups();
            } catch (error) {
                showErrorInfo(error.response?.data?.message || error.message);
            } finally {
                this.loading = false;
            }
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
        </div>
        <div class="scrolling content">
            <div class="ui info message">
                <p>Select customers to add to this group:</p>
            </div>
            <div class="ui fluid icon input" style="margin-bottom: 15px">
                <input type="text" v-model="searchQuery" @input="handleSearch" placeholder="Search customers...">
                <i class="search icon"></i>
            </div>
            <div class="ui middle aligned divided selection list" style="max-height: 400px; overflow-y: auto">
                <div class="item" v-for="customer in customers" :key="customer.id" 
                     @click="toggleCustomer(customer.id)" style="cursor: pointer">
                    <div class="right floated content">
                        <div class="ui toggle checkbox">
                            <input type="checkbox" :checked="isCustomerInGroup(customer.id)" @click.stop="toggleCustomer(customer.id)">
                            <label></label>
                        </div>
                    </div>
                    <i class="large user circle icon"></i>
                    <div class="content">
                        <div class="header">{{ customer.full_name || customer.phone }}</div>
                        <div class="description">{{ customer.phone }}</div>
                    </div>
                </div>
            </div>
            <div class="ui message" v-if="customers.length === 0">
                No customers available. Add customers first.
            </div>
        </div>
        <div class="actions">
            <button class="ui blue button" :class="{loading: loading}" @click="saveMembers">
                <i class="save icon"></i> Save Members ({{ selectedCustomerIds.length }} selected)
            </button>
        </div>
    </div>
    `
}
