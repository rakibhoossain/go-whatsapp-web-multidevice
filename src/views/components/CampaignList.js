export default {
    name: 'CampaignList',
    data() {
        return {
            loading: false,
            campaigns: [],
            templates: [],
            customers: [],
            groups: [],
            form: {
                name: '',
                template_id: '',
                customer_ids: [],
                group_ids: [],
                scheduled_at: ''
            },
            editingId: null,
            selectedCampaign: null,
            statsInterval: null,
            page: 1,
            pageSize: 10,
            total: 0,
            searchQuery: '',
            searchTimeout: null,
            customersPage: 1,
            customersLoading: false,
            customersHasMore: true
        }
    },
    computed: {
        statusColors() {
            return {
                'draft': 'grey',
                'running': 'green',
                'paused': 'yellow'
            };
        },
        totalPages() {
            return Math.ceil(this.total / this.pageSize);
        }
    },
    methods: {
        async openModal() {
            $('#modalCampaignList').modal('show');
            await this.loadCampaigns();
        },
        async loadCampaigns() {
            try {
                this.loading = true;
                const response = await window.http.get(`/campaign/campaigns?page=${this.page}&page_size=${this.pageSize}`);
                this.campaigns = response.data.results.campaigns || [];
                this.total = response.data.results.total || 0;
            } catch (error) {
                showErrorInfo(error.response?.data?.message || error.message);
            } finally {
                this.loading = false;
            }
        },
        async loadFormData() {
            try {
                const [templatesRes, customersRes, groupsRes] = await Promise.all([
                    window.http.get('/campaign/templates'),
                    window.http.get('/campaign/customers?page_size=1000'),
                    window.http.get('/campaign/groups')
                ]);
                this.templates = templatesRes.data.results.templates || [];
                this.templates = templatesRes.data.results.templates || [];
                // Initialize customers with pagination
                this.customers = [];
                this.customersPage = 1;
                this.customersHasMore = true;
                await this.loadCustomers();
                this.groups = groupsRes.data.results.groups || [];
                this.groups = groupsRes.data.results.groups || [];
            } catch (error) {
                console.error('Failed to load form data:', error);
            }
        },
        async openCreateModal() {
            this.resetForm();
            this.editingId = null;
            await this.loadFormData();
            this.$nextTick(() => {
                $('.campaign-dropdown').dropdown('clear');
            });
            $('#modalCampaignForm').modal('show');
        },
        async openEditModal(campaign) {
            await this.loadFormData();
            // Fetch full campaign data to get target IDs
            try {
                const response = await window.http.get(`/campaign/campaigns/${campaign.id}`);
                const fullCampaign = response.data.results;
                this.form = {
                    name: fullCampaign.name,
                    template_id: fullCampaign.template_id,
                    customer_ids: fullCampaign.customer_ids || [],
                    group_ids: fullCampaign.group_ids || [],
                    scheduled_at: fullCampaign.scheduled_at ? new Date(fullCampaign.scheduled_at).toISOString().slice(0, 16) : ''
                };
                this.editingId = fullCampaign.id;
                $('#modalCampaignForm').modal('show');
            } catch (error) {
                showErrorInfo(error.response?.data?.message || error.message);
            }
        },
        resetForm() {
            this.form = { name: '', template_id: '', customer_ids: [], group_ids: [], scheduled_at: '' };
        },
        async submitForm() {
            if (!this.form.name.trim()) {
                showErrorInfo('Campaign name is required');
                return;
            }
            if (!this.form.template_id) {
                showErrorInfo('Please select a template');
                return;
            }
            if (this.form.customer_ids.length === 0 && this.form.group_ids.length === 0) {
                showErrorInfo('Please select at least one customer or group');
                return;
            }
            try {
                this.loading = true;
                const payload = {
                    name: this.form.name,
                    template_id: this.form.template_id,
                    customer_ids: this.form.customer_ids,
                    group_ids: this.form.group_ids,
                    scheduled_at: this.form.scheduled_at ? new Date(this.form.scheduled_at).toISOString() : null
                };

                if (this.editingId) {
                    await window.http.put(`/campaign/campaigns/${this.editingId}`, payload);
                    showSuccessInfo('Campaign updated');
                } else {
                    await window.http.post('/campaign/campaigns', payload);
                    showSuccessInfo('Campaign created');
                }
                $('#modalCampaignForm').modal('hide');
                await this.loadCampaigns();
            } catch (error) {
                showErrorInfo(error.response?.data?.message || error.message);
            } finally {
                this.loading = false;
            }
        },
        async deleteCampaign(id) {
            if (!confirm('Are you sure you want to delete this campaign?')) return;
            try {
                await window.http.delete(`/campaign/campaigns/${id}`);
                showSuccessInfo('Campaign deleted');
                await this.loadCampaigns();
            } catch (error) {
                showErrorInfo(error.response?.data?.message || error.message);
            }
        },
        async startCampaign(id) {
            if (!confirm('Start sending messages for this campaign?')) return;
            try {
                this.loading = true;
                await window.http.post(`/campaign/campaigns/${id}/start`);
                showSuccessInfo('Campaign started! Messages are being sent.');
                await this.loadCampaigns();
            } catch (error) {
                showErrorInfo(error.response?.data?.message || error.message);
            } finally {
                this.loading = false;
            }
        },
        async pauseCampaign(id) {
            try {
                await window.http.post(`/campaign/campaigns/${id}/pause`);
                showSuccessInfo('Campaign paused');
                await this.loadCampaigns();
            } catch (error) {
                showErrorInfo(error.response?.data?.message || error.message);
            }
        },
        async openStatsModal(campaign) {
            this.selectedCampaign = campaign;
            await this.refreshStats();
            $('#modalCampaignStats').modal('show');
            // Auto-refresh stats every 5s when modal is open
            this.statsInterval = setInterval(() => this.refreshStats(), 5000);
        },
        closeStatsModal() {
            if (this.statsInterval) {
                clearInterval(this.statsInterval);
                this.statsInterval = null;
            }
        },
        async refreshStats() {
            if (!this.selectedCampaign) return;
            try {
                const response = await window.http.get(`/campaign/campaigns/${this.selectedCampaign.id}`);
                this.selectedCampaign = response.data.results;
            } catch (error) {
                console.error('Failed to refresh stats:', error);
            }
        },
        getProgress(stats) {
            if (!stats || stats.total_messages === 0) return 0;
            return Math.round(((stats.sent_messages + stats.failed_messages) / stats.total_messages) * 100);
        },
        formatDate(date) {
            if (!date) return '-';
            return new Date(date).toLocaleString();
        },
        toggleCustomer(customerId) {
            const index = this.form.customer_ids.indexOf(customerId);
            if (index > -1) {
                this.form.customer_ids.splice(index, 1);
            } else {
                this.form.customer_ids.push(customerId);
            }
        },
        toggleGroup(groupId) {
            const index = this.form.group_ids.indexOf(groupId);
            if (index > -1) {
                this.form.group_ids.splice(index, 1);
            } else {
                this.form.group_ids.push(groupId);
            }
        },
        truncate(text, length = 100) {
            return text.length > length ? text.substring(0, length) + '...' : text;
        },
        nextPage() {
            if (this.page < this.totalPages) {
                this.page++;
                this.loadCampaigns();
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
                const response = await window.http.get(url);
                const newCustomers = response.data.results.customers || [];

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
        handleCustomerScroll(e) {
            const { scrollTop, scrollHeight, clientHeight } = e.target;
            if (scrollTop + clientHeight >= scrollHeight - 50) {
                this.loadCustomers();
            }
        },
        handleSearch() {
            clearTimeout(this.searchTimeout);
            this.searchTimeout = setTimeout(() => {
                this.loadCustomers(true);
            }, 500);
        },
        prevPage() {
            if (this.page > 1) {
                this.page--;
                this.loadCampaigns();
            }
        }
    },
    beforeUnmount() {
        this.closeStatsModal();
    },
    template: `
    <div class="red card" @click="openModal" style="cursor: pointer">
        <div class="content">
            <a class="ui red right ribbon label">Campaign</a>
            <div class="header">Campaigns</div>
            <div class="description">
                Create and manage message campaigns
            </div>
        </div>
    </div>
    
    <!-- Campaigns List Modal -->
    <div class="ui large modal" id="modalCampaignList">
        <i class="close icon"></i>
        <div class="header">
            <i class="bullhorn icon"></i> Campaigns
            <button class="ui green right floated button" @click.stop="openCreateModal">
                <i class="plus icon"></i> New Campaign
            </button>
        </div>
        <div class="scrolling content">
            <div class="ui active inverted dimmer" v-if="loading">
                <div class="ui loader"></div>
            </div>
            <table class="ui celled striped table">
                <thead>
                    <tr>
                        <th>Name</th>
                        <th>Status</th>
                        <th>Progress</th>
                        <th>Created</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>
                    <tr v-for="campaign in campaigns" :key="campaign.id">
                        <td><strong>{{ campaign.name }}</strong></td>
                        <td>
                            <span :class="'ui ' + statusColors[campaign.status] + ' label'">
                                {{ campaign.status.toUpperCase() }}
                            </span>
                        </td>
                        <td>
                            <div class="ui small progress" :class="{green: campaign.stats}" style="margin: 0" v-if="campaign.stats">
                                <div class="bar" :style="{width: getProgress(campaign.stats) + '%'}">
                                    <div class="progress">{{ getProgress(campaign.stats) }}%</div>
                                </div>
                            </div>
                            <span v-else>-</span>
                        </td>
                        <td>{{ formatDate(campaign.created_at) }}</td>
                        <td>
                            <div class="ui mini buttons">
                                <button class="ui blue button" @click.stop="openStatsModal(campaign)" title="Stats">
                                    <i class="chart bar icon"></i>
                                </button>
                                <button class="ui green button" v-if="campaign.status === 'draft' || campaign.status === 'paused'" 
                                        @click.stop="startCampaign(campaign.id)" title="Start">
                                    <i class="play icon"></i>
                                </button>
                                <button class="ui yellow button" v-if="campaign.status === 'running'" 
                                        @click.stop="pauseCampaign(campaign.id)" title="Pause">
                                    <i class="pause icon"></i>
                                </button>
                                <button class="ui orange button" v-if="campaign.status === 'draft' || campaign.status === 'paused'" 
                                        @click.stop="openEditModal(campaign)" title="Edit">
                                    <i class="edit icon"></i>
                                </button>
                                <button class="ui red button" v-if="campaign.status !== 'running'" 
                                        @click.stop="deleteCampaign(campaign.id)" title="Delete">
                                    <i class="trash icon"></i>
                                </button>
                            </div>
                        </td>
                    </tr>
                </tbody>
            </table>
            
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

            <div class="ui message" v-if="campaigns.length === 0 && !loading">
                No campaigns created yet. Create a campaign to start sending messages.
            </div>
        </div>
    </div>
    
    <!-- Campaign Form Modal -->
    <div class="ui large modal" id="modalCampaignForm">
        <i class="close icon"></i>
        <div class="header">{{ editingId ? 'Edit Campaign' : 'Create Campaign' }}</div>
        <div class="scrolling content">
            <form class="ui form">
                <div class="required field">
                    <label>Campaign Name</label>
                    <input v-model="form.name" type="text" placeholder="Summer Promotion 2024">
                </div>
                
                <div class="required field">
                    <label>Message Template</label>
                    <select v-model="form.template_id" class="ui dropdown campaign-dropdown">
                        <option value="">Select Template</option>
                        <option v-for="t in templates" :key="t.id" :value="t.id">{{ t.name }}</option>
                    </select>
                </div>
                
                <div class="field">
                    <label>Schedule (Optional)</label>
                    <input v-model="form.scheduled_at" type="datetime-local">
                    <small>Leave empty to start manually</small>
                </div>
                
                <div class="ui segment">
                    <h4 class="ui header">Target Audience</h4>
                    
                    <div class="field">
                        <label>Select Groups</label>
                        <div class="ui middle aligned divided selection list" style="max-height: 150px; overflow-y: auto">
                            <div class="item" v-for="group in groups" :key="group.id" 
                                 @click="toggleGroup(group.id)" style="cursor: pointer">
                                <div class="right floated content">
                                    <div class="ui checkbox">
                                        <input type="checkbox" :checked="form.group_ids.includes(group.id)">
                                        <label></label>
                                    </div>
                                </div>
                                <i class="folder icon"></i>
                                <div class="content">
                                    {{ group.name }} <span class="ui mini label">{{ group.customer_count || 0 }} members</span>
                                </div>
                            </div>
                        </div>
                    </div>
                    
                    <div class="field">
                        <label>Select Individual Customers</label>
                        <div class="ui fluid icon input" style="margin-bottom: 10px">
                            <input type="text" v-model="searchQuery" @input="handleSearch" placeholder="Search customers...">
                            <i class="search icon"></i>
                        </div>
                        <div class="ui middle aligned divided selection list" style="max-height: 200px; overflow-y: auto" @scroll="handleCustomerScroll">
                            <div class="item" v-for="customer in customers" :key="customer.id" 
                                 @click="toggleCustomer(customer.id)" style="cursor: pointer">
                                <div class="right floated content">
                                    <div class="ui checkbox">
                                        <input type="checkbox" :checked="form.customer_ids.includes(customer.id)">
                                        <label></label>
                                    </div>
                                </div>
                                <i class="user icon"></i>
                                <div class="content">
                                    {{ customer.full_name || customer.phone }} 
                                    <small v-if="customer.full_name">({{ customer.phone }})</small>
                                </div>
                            </div>
                        </div>
                    </div>
                    
                    <div class="ui info message">
                        <p>Selected: {{ form.group_ids.length }} groups, {{ form.customer_ids.length }} individual customers</p>
                    </div>
                </div>
            </form>
        </div>
        <div class="actions">
            <button class="ui positive button" :class="{loading: loading}" @click="submitForm">
                <i class="check icon"></i> {{ editingId ? 'Update' : 'Create' }} Campaign
            </button>
        </div>
    </div>
    
    <!-- Campaign Stats Modal -->
    <div class="ui modal" id="modalCampaignStats" @hide="closeStatsModal">
        <i class="close icon" @click="closeStatsModal"></i>
        <div class="header">
            <i class="chart bar icon"></i> Campaign Stats - {{ selectedCampaign?.name }}
        </div>
        <div class="content" v-if="selectedCampaign">
            <div class="ui four statistics">
                <div class="statistic">
                    <div class="value">{{ selectedCampaign.stats?.total_messages || 0 }}</div>
                    <div class="label">Total</div>
                </div>
                <div class="blue statistic">
                    <div class="value">{{ selectedCampaign.stats?.pending_messages || 0 }}</div>
                    <div class="label">Pending</div>
                </div>
                <div class="green statistic">
                    <div class="value">{{ selectedCampaign.stats?.sent_messages || 0 }}</div>
                    <div class="label">Sent</div>
                </div>
                <div class="red statistic">
                    <div class="value">{{ selectedCampaign.stats?.failed_messages || 0 }}</div>
                    <div class="label">Failed</div>
                </div>
            </div>
            
            <div class="ui segment" style="margin-top: 20px">
                <div class="ui indicating progress" :data-percent="getProgress(selectedCampaign.stats)">
                    <div class="bar" :style="{width: getProgress(selectedCampaign.stats) + '%'}">
                        <div class="progress">{{ getProgress(selectedCampaign.stats) }}%</div>
                    </div>
                    <div class="label">Campaign Progress</div>
                </div>
            </div>
            
            <table class="ui definition table">
                <tbody>
                    <tr>
                        <td>Status</td>
                        <td><span :class="'ui ' + statusColors[selectedCampaign.status] + ' label'">{{ selectedCampaign.status.toUpperCase() }}</span></td>
                    </tr>
                    <tr>
                        <td>Started At</td>
                        <td>{{ formatDate(selectedCampaign.started_at) }}</td>
                    </tr>
                    <tr>
                        <td>Completed At</td>
                        <td>{{ formatDate(selectedCampaign.completed_at) }}</td>
                    </tr>
                </tbody>
            </table>
        </div>
    </div>
    `
}
