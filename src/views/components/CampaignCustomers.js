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
            searchTimeout: null
        }
    },
    watch: {
        searchQuery() {
            if (this.searchTimeout) {
                clearTimeout(this.searchTimeout);
            }
            this.searchTimeout = setTimeout(() => {
                this.page = 1; // Reset to page 1 on search
                this.loadCustomers();
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
            await this.loadCustomers();
        },
        async loadCustomers() {
            try {
                this.loading = true;
                this.selectedIds = []; // Clear selection on reload
                const response = await window.http.get(`/campaign/customers?page=${this.page}&page_size=${this.pageSize}&search=${encodeURIComponent(this.searchQuery)}`);
                this.customers = response.data.results.customers || [];
                this.total = response.data.results.total;
            } catch (error) {
                showErrorInfo(error.response?.data?.message || error.message);
            } finally {
                this.loading = false;
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
                await this.loadCustomers();
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
                await this.loadCustomers();

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
                await this.loadCustomers();
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
                await this.loadCustomers();
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
                await this.loadCustomers();
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
                await this.loadCustomers();
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
        nextPage() {
            if (this.page < this.totalPages) {
                this.page++;
                this.loadCustomers();
            }
        },
        prevPage() {
            if (this.page > 1) {
                this.page--;
                this.loadCustomers();
            }
        }
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
                <div class="ui fluid mini icon input" style="width: 200px;">
                    <input type="text" placeholder="Search..." v-model="searchQuery" @keyup.enter="loadCustomers">
                    <i class="search link icon" @click="loadCustomers"></i>
                </div>
            </div>
        </div>
        <div class="scrolling content">
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
