export default {
    name: 'CampaignCustomers',
    data() {
        return {
            loading: false,
            customers: [],
            total: 0,
            page: 1,
            pageSize: 20,
            form: {
                phone: '',
                full_name: '',
                company: '',
                country: '',
                gender: '',
                birth_year: ''
            },
            editingId: null,
            importErrors: [],
            searchQuery: '',
            selectedIds: [],
            searchTimeout: null,
            hasMore: true,
            loadingMore: false
        }
    },
    watch: {
        searchQuery() {
            if (this.searchTimeout) {
                clearTimeout(this.searchTimeout);
            }
            this.searchTimeout = setTimeout(() => {
                this.searchTimeout = setTimeout(() => {
                    this.loadCustomers(true);
                }, 500);
            }, 500);
        }
    },
    computed: {
        totalPages() {
            return Math.ceil(this.total / this.pageSize);
        }
    },
    methods: {
        async openModal() {
            $('#modalCampaignCustomers').modal('show');
            $('#modalCampaignCustomers').modal('show');
            await this.loadCustomers(true);
        },
        async loadCustomers(reset = false) {
            if (this.loadingMore) return;

            if (reset) {
                this.page = 1;
                this.customers = [];
                this.hasMore = true;
                this.selectedIds = [];
            }

            if (!this.hasMore) return;

            try {
                if (this.customers.length === 0) {
                    this.loading = true;
                } else {
                    this.loadingMore = true;
                }

                const response = await window.http.get(`/campaign/customers?page=${this.page}&page_size=${this.pageSize}&search=${encodeURIComponent(this.searchQuery)}`);
                const newCustomers = response.data.results.customers || [];
                this.total = response.data.results.total;

                if (newCustomers.length < this.pageSize) {
                    this.hasMore = false;
                }

                if (reset) {
                    this.customers = newCustomers;
                } else {
                    this.customers = [...this.customers, ...newCustomers];
                }

                if (newCustomers.length > 0) {
                    this.page++;
                }
            } catch (error) {
                showErrorInfo(error.response?.data?.message || error.message);
            } finally {
                this.loading = false;
                this.loadingMore = false;
            }
        },
        handleScroll(e) {
            const { scrollTop, scrollHeight, clientHeight } = e.target;
            // Load more when scrolled near bottom (50px buffer)
            if (scrollTop + clientHeight >= scrollHeight - 50) {
                this.loadCustomers();
            }
        },
        async deleteSelectedCustomers() {
            if (this.selectedIds.length === 0) return;
            if (!confirm(`Are you sure you want to delete ${this.selectedIds.length} customers?`)) return;

            try {
                this.loading = true;
                await window.http.post('/campaign/customers/bulk-delete', { ids: this.selectedIds });
                showSuccessInfo('Customers deleted');
                this.selectedIds = [];
                await this.loadCustomers(true);
            } catch (error) {
                showErrorInfo(error.response?.data?.message || error.message);
            } finally {
                this.loading = false;
            }
        },
        async validateSelectedCustomers() {
            if (this.selectedIds.length === 0) return;
            if (!confirm(`Are you sure you want to validate ${this.selectedIds.length} customers?`)) return;

            try {
                this.loading = true;
                await window.http.post('/campaign/customers/validate-bulk', { ids: this.selectedIds });
                showSuccessInfo('Validation started');
                this.selectedIds = [];
                await this.loadCustomers(true);
            } catch (error) {
                showErrorInfo(error.response?.data?.message || error.message);
            } finally {
                this.loading = false;
            }
        },
        toggleSelectAll() {
            if (this.selectedIds.length === this.customers.length) {
                this.selectedIds = [];
            } else {
                this.selectedIds = this.customers.map(c => c.id);
            }
        },
        isAllSelected() {
            return this.customers.length > 0 && this.selectedIds.length === this.customers.length;
        },
        // initDataTable removed as it is now integrated into loadCustomers
        openCreateModal() {
            this.resetForm();
            this.editingId = null;
            $('#modalCampaignCustomerForm').modal('show');
        },
        openEditModal(customer) {
            this.form = {
                phone: customer.phone,
                full_name: customer.full_name || '',
                company: customer.company || '',
                country: customer.country || '',
                gender: customer.gender || '',
                birth_year: customer.birth_year || ''
            };
            this.editingId = customer.id;
            $('#modalCampaignCustomerForm').modal('show');
        },
        resetForm() {
            this.form = { phone: '', full_name: '', company: '', country: '', gender: '', birth_year: '' };
        },
        async submitForm() {
            if (!this.form.phone.startsWith('+')) {
                showErrorInfo('Phone must start with +');
                return;
            }
            // Validate phone format (no leading 0 after +)
            const phoneNum = this.form.phone.substring(1);
            if (phoneNum.startsWith('0')) {
                showErrorInfo('Phone must be in international format (no leading 0 after +)');
                return;
            }
            try {
                this.loading = true;
                const payload = {
                    phone: this.form.phone,
                    full_name: this.form.full_name || null,
                    company: this.form.company || null,
                    country: this.form.country || null,
                    gender: this.form.gender || null,
                    birth_year: this.form.birth_year ? parseInt(this.form.birth_year) : null
                };

                let response;
                if (this.editingId) {
                    response = await window.http.put(`/campaign/customers/${this.editingId}`, payload);
                    showSuccessInfo('Customer updated');
                } else {
                    response = await window.http.post('/campaign/customers', payload);
                    showSuccessInfo('Customer created');
                }

                $('#modalCampaignCustomerForm').modal('hide');
                await this.loadCustomers(true);

                // If created new customer, trigger checking immediately for better UX
                if (!this.editingId && response && response.data && response.data.results) {
                    const newId = response.data.results.id;
                    if (newId) {
                        // Trigger async validation (don't await)
                        this.validateCustomer(newId);
                    }
                }
            } catch (error) {
                showErrorInfo(error.response?.data?.message || error.message);
            } finally {
                this.loading = false;
            }
        },
        async deleteCustomer(id) {
            if (!confirm('Are you sure you want to delete this customer?')) return;
            try {
                await window.http.delete(`/campaign/customers/${id}`);
                showSuccessInfo('Customer deleted');
                await this.loadCustomers(true);
            } catch (error) {
                showErrorInfo(error.response?.data?.message || error.message);
            }
        },
        async validateCustomer(id) {
            try {
                // Don't set global loading here to avoid freezing UI for background checks
                await window.http.post(`/campaign/customers/${id}/validate`);
                // Only reload if this was an explicit user action (checked by loading state or passed arg)
                // But for auto-check after create, we might want to reload quietly or just let the user refresh
                // For now, let's reload to update the status icon
                // Removed reload to prevent jumping list in infinite scroll
                // Just let the user refresh manually or rely on local state if we implemented it
                // But since status is in DB, let's just find the customer and update it locally if possible?
                // For now, simpler to not reload entire list as it resets scroll position
                // Alternatively, we can just fetch this specific customer again if we had an endpoint
                // Or just assume it's pending->validated transition attempted.
                // Let's reload only if list is short to avoid UX jar, 
                // but for infinite scroll, full reload is bad UX. 
                // Let's try to update just this item in the array if we can.
                // Since this is a simple app, let's doing nothing for now or maybe show a toast.
                // The user can pull/scroll? No, infinite scroll doesn't usually have pull-to-refresh.
                // Let's call loadCustomers(true) only if we really must.
                // Ideally, we'd update the local object.
                const customerIndex = this.customers.findIndex(c => c.id === id);
                if (customerIndex !== -1) {
                    // Optimistic update or quick fetch?
                    // Let's leave it, the background generic check might be running.
                    // Or we could fetch just this customer.
                    // For now, to match previous behavior but avoid scroll jump:
                    // We simply don't reload. The user will see status on next open or refresh.
                }
            } catch (error) {
                console.error("Validation failed:", error);
                // Don't show error to user for auto-checks
            }
        },
        async validatePendingCustomers() {
            if (!confirm('Start validation for pending customers? This will check up to 1000 pending numbers.')) return;
            try {
                this.loading = true;
                // Use new bulk endpoint
                await window.http.post('/campaign/customers/validate-pending');
                showSuccessInfo('Validation check started');
                await this.loadCustomers(true);
            } catch (error) {
                showErrorInfo(error.response?.data?.message || error.message);
            } finally {
                this.loading = false;
            }
        },
        openImportModal() {
            this.importErrors = [];
            $('#modalCampaignCustomerImport').modal('show');
        },
        downloadTemplate() {
            const headers = ['phone', 'full_name', 'company', 'country', 'gender', 'birth_year'];
            const sample = ['+1234567890', 'John Doe', 'Acme Corp', 'USA', 'male', '1990'];
            const csvContent = "data:text/csv;charset=utf-8," +
                headers.join(",") + "\n" +
                sample.join(",");

            const encodedUri = encodeURI(csvContent);
            const link = document.createElement("a");
            link.setAttribute("href", encodedUri);
            link.setAttribute("download", "customer_import_template.csv");
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
        },
        async handleImport() {
            const fileInput = document.getElementById('csvFileInput');
            if (!fileInput.files[0]) {
                showErrorInfo('Please select a CSV file');
                return;
            }
            try {
                this.loading = true;
                const formData = new FormData();
                formData.append('file', fileInput.files[0]);
                const response = await window.http.post('/campaign/customers/import', formData);
                const result = response.data.results;
                showSuccessInfo(`Imported ${result.imported} customers`);
                this.importErrors = result.errors || [];
                if (this.importErrors.length === 0) {
                    $('#modalCampaignCustomerImport').modal('hide');
                }
                await this.loadCustomers(true);
            } catch (error) {
                showErrorInfo(error.response?.data?.message || error.message);
            } finally {
                this.loading = false;
            }
        },
        getStatusColor(status) {
            return {
                'pending': 'grey',
                'valid': 'green',
                'invalid': 'red'
            }[status] || 'grey';
        },
    },
    template: `
    <div class="teal card" @click="openModal" style="cursor: pointer">
        <div class="content">
            <a class="ui teal right ribbon label">Campaign</a>
            <div class="header">Customers</div>
            <div class="description">
                Manage campaign customers
            </div>
        </div>
    </div>
    
    <!-- Customers List Modal -->
    <div class="ui large modal" id="modalCampaignCustomers">
        <i class="close icon"></i>
        <div class="header">
            <i class="users icon"></i> Campaign Customers
            <div class="ui buttons right floated" style="margin-left: 10px">
                <button class="ui red button" v-if="selectedIds.length > 0" @click.stop="deleteSelectedCustomers" style="margin-right: 5px">
                    <i class="trash icon"></i> Delete ({{ selectedIds.length }})
                </button>
                <button class="ui purple button" v-if="selectedIds.length > 0" @click.stop="validateSelectedCustomers" style="margin-right: 5px">
                    <i class="check double icon"></i> Validate ({{ selectedIds.length }})
                </button>
                <button class="ui orange button" @click.stop="validatePendingCustomers" style="margin-right: 5px">
                    <i class="sync icon"></i> Check Again
                </button>
                <button class="ui green button" @click.stop="openCreateModal" style="margin-right: 5px">
                    <i class="plus icon"></i> Add
                </button>
                <button class="ui blue button" @click.stop="openImportModal">
                    <i class="upload icon"></i> Import CSV
                </button>
            </div>
            <div style="padding-top: 0.8em !important;">
                <div class="ui fluid left icon input" style="min-width: 250px;">
                    <input type="text" placeholder="Search by name or phone..." v-model="searchQuery">
                    <i class="search icon"></i>
                </div>
            </div>
        </div>
        <div class="scrolling content" style="max-height: 600px; overflow-y: auto" @scroll="handleScroll">
            <div class="ui active inverted dimmer" v-if="loading">
                <div class="ui loader"></div>
            </div>
            <table class="ui celled striped table" id="campaign_customers_table">
                <thead>
                    <tr>
                        <th class="collapsing">
                            <div class="ui checkbox">
                                <input type="checkbox" :checked="isAllSelected()" @change="toggleSelectAll">
                                <label></label>
                            </div>
                        </th>
                        <th>Phone</th>
                        <th>Name</th>
                        <th>Company</th>
                        <th>Status</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>
                    <tr v-for="customer in customers" :key="customer.id">
                        <td>
                            <div class="ui checkbox">
                                <input type="checkbox" :value="customer.id" v-model="selectedIds">
                                <label></label>
                            </div>
                        </td>
                        <td>{{ customer.phone }}</td>
                        <td>{{ customer.full_name || '-' }}</td>
                        <td>{{ customer.company || '-' }}</td>
                        <td>
                            <span :class="'ui mini ' + getStatusColor(customer.phone_valid) + ' label'" title="Phone Format">
                                Phone: {{ customer.phone_valid || 'pending' }}
                            </span>
                            <span :class="'ui mini ' + getStatusColor(customer.whatsapp_exists) + ' label'" title="WhatsApp">
                                WA: {{ customer.whatsapp_exists || 'pending' }}
                            </span>
                            <i class="green check circle icon" v-if="customer.is_ready" title="Ready to send"></i>
                        </td>
                        <td>
                            <div class="ui mini buttons">
                                <button class="ui teal button" @click.stop="validateCustomer(customer.id)" title="Validate" style="margin-right: 2px;">
                                    <i class="check icon"></i>
                                </button>
                                <button class="ui yellow button" @click.stop="openEditModal(customer)" title="Edit" style="margin-right: 2px;">
                                    <i class="edit icon"></i>
                                </button>
                                <button class="ui red button" @click.stop="deleteCustomer(customer.id)" title="Delete">
                                    <i class="trash icon"></i>
                                </button>
                            </div>
                        </td>
                    </tr>
                </tbody>
            </table>
            <div class="ui message" v-if="customers.length === 0 && !loading">
                No customers found. Add customers manually or import from CSV.
            </div>
            
            <!-- Pagination -->
            <!-- Loader for infinite scroll -->
            <div class="ui center aligned basic segment" v-if="loadingMore">
                 <div class="ui active centered inline loader"></div>
            </div>
            
            <div class="ui centered grid" v-if="customers.length > 0 && !hasMore">
                 <div class="row">
                     <div class="column center aligned">
                         <span class="ui tiny grey text">No more customers</span>
                     </div>
                 </div>
            </div>
        </div>
    </div>
    
    <!-- Customer Form Modal -->
    <div class="ui small modal" id="modalCampaignCustomerForm">
        <i class="close icon"></i>
        <div class="header">{{ editingId ? 'Edit Customer' : 'Add Customer' }}</div>
        <div class="content">
            <form class="ui form">
                <div class="required field">
                    <label>Phone (International format, e.g. +8801234567890)</label>
                    <input v-model="form.phone" type="text" placeholder="+8801234567890">
                </div>
                <div class="field">
                    <label>Full Name</label>
                    <input v-model="form.full_name" type="text" placeholder="John Doe">
                </div>
                <div class="field">
                    <label>Company</label>
                    <input v-model="form.company" type="text" placeholder="Acme Inc.">
                </div>
                <div class="two fields">
                    <div class="field">
                        <label>Country</label>
                        <input v-model="form.country" type="text" placeholder="Bangladesh">
                    </div>
                    <div class="field">
                        <label>Gender</label>
                        <select v-model="form.gender" class="ui dropdown">
                            <option value="">Select Gender</option>
                            <option value="male">Male</option>
                            <option value="female">Female</option>
                            <option value="other">Other</option>
                        </select>
                    </div>
                </div>
                <div class="field">
                    <label>Birth Year</label>
                    <input v-model="form.birth_year" type="number" placeholder="1990" min="1900" max="2020">
                </div>
            </form>
        </div>
        <div class="actions">
            <button class="ui positive button" :class="{loading: loading}" @click="submitForm">
                <i class="check icon"></i> Save
            </button>
        </div>
    </div>
    
    <!-- Import Modal -->
    <div class="ui small modal" id="modalCampaignCustomerImport">
        <i class="close icon"></i>
        <div class="header">Import Customers from CSV</div>
        <div class="content">
            <div class="ui info message">
                <p>CSV must have a <b>phone</b> column. Optional columns: <b>name</b>, <b>company</b>, <b>country</b>, <b>gender</b>, <b>birth_year</b></p>
                <div style="margin-top: 10px">
                    <button class="ui tiny blue button" @click.prevent="downloadTemplate">
                        <i class="download icon"></i> Download Template
                    </button>
                </div>
            </div>
            <div class="ui form">
                <div class="field">
                    <label>Select CSV File</label>
                    <input type="file" id="csvFileInput" accept=".csv">
                </div>
            </div>
            <div class="ui error message" v-if="importErrors.length > 0">
                <div class="header">Import Errors</div>
                <ul class="list">
                    <li v-for="err in importErrors">{{ err }}</li>
                </ul>
            </div>
        </div>
        <div class="actions">
            <button class="ui green button" :class="{loading: loading}" @click="handleImport">
                <i class="upload icon"></i> Import
            </button>
        </div>
    </div>
    `
}
