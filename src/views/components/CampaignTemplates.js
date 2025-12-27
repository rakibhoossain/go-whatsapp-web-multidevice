export default {
    name: 'CampaignTemplates',
    data() {
        return {
            loading: false,
            templates: [],
            form: {
                name: '',
                content: ''
            },
            editingId: null,
            previewText: '',
            page: 1,
            pageSize: 10,
            total: 0
        }
    },
    computed: {
        placeholders() {
            return ['[NAME]', '[PHONE]', '[COUNTRY]', '[GROUP]', '[COMPANY]'];
        },
        totalPages() {
            return Math.ceil(this.total / this.pageSize);
        }
    },
    methods: {
        async openModal() {
            $('#modalCampaignTemplates').modal('show');
            await this.loadTemplates();
        },
        async loadTemplates() {
            try {
                this.loading = true;
                const response = await window.http.get(`/campaign/templates?page=${this.page}&page_size=${this.pageSize}`);
                this.templates = response.data.results.templates || [];
                this.total = response.data.results.total || 0;
            } catch (error) {
                showErrorInfo(error.response?.data?.message || error.message);
            } finally {
                this.loading = false;
            }
        },
        openCreateModal() {
            this.resetForm();
            this.editingId = null;
            this.previewText = '';
            $('#modalCampaignTemplateForm').modal('show');
        },
        openEditModal(template) {
            this.form = {
                name: template.name,
                content: template.content
            };
            this.editingId = template.id;
            this.updatePreview();
            $('#modalCampaignTemplateForm').modal('show');
        },
        resetForm() {
            this.form = { name: '', content: '' };
        },
        insertPlaceholder(placeholder) {
            this.form.content += placeholder;
            this.updatePreview();
        },
        updatePreview() {
            let preview = this.form.content;
            preview = preview.replace(/\[NAME\]/g, 'John Doe');
            preview = preview.replace(/\[PHONE\]/g, '+8801234567890');
            preview = preview.replace(/\[COUNTRY\]/g, 'Bangladesh');
            preview = preview.replace(/\[GROUP\]/g, 'VIP Customers');
            preview = preview.replace(/\[COMPANY\]/g, 'Acme Inc.');
            this.previewText = preview;
        },
        async submitForm() {
            if (!this.form.name.trim()) {
                showErrorInfo('Template name is required');
                return;
            }
            if (!this.form.content.trim()) {
                showErrorInfo('Template content is required');
                return;
            }
            try {
                this.loading = true;
                const payload = {
                    name: this.form.name,
                    content: this.form.content
                };

                if (this.editingId) {
                    await window.http.put(`/campaign/templates/${this.editingId}`, payload);
                    showSuccessInfo('Template updated');
                } else {
                    await window.http.post('/campaign/templates', payload);
                    showSuccessInfo('Template created');
                }
                $('#modalCampaignTemplateForm').modal('hide');
                await this.loadTemplates();
            } catch (error) {
                showErrorInfo(error.response?.data?.message || error.message);
            } finally {
                this.loading = false;
            }
        },
        async deleteTemplate(id) {
            if (!confirm('Are you sure you want to delete this template?')) return;
            try {
                await window.http.delete(`/campaign/templates/${id}`);
                showSuccessInfo('Template deleted');
                await this.loadTemplates();
            } catch (error) {
                showErrorInfo(error.response?.data?.message || error.message);
            }
        },
        truncate(text, length = 100) {
            return text.length > length ? text.substring(0, length) + '...' : text;
        },
        nextPage() {
            if (this.page < this.totalPages) {
                this.page++;
                this.loadTemplates();
            }
        },
        prevPage() {
            if (this.page > 1) {
                this.page--;
                this.loadTemplates();
            }
        }
    },
    template: `
    <div class="orange card" @click="openModal" style="cursor: pointer">
        <div class="content">
            <a class="ui orange right ribbon label">Campaign</a>
            <div class="header">Message Templates</div>
            <div class="description">
                Create reusable message templates
            </div>
        </div>
    </div>
    
    <!-- Templates List Modal -->
    <div class="ui modal" id="modalCampaignTemplates">
        <i class="close icon"></i>
        <div class="header">
            <i class="file alternate icon"></i> Message Templates
            <button class="ui green right floated button" @click.stop="openCreateModal">
                <i class="plus icon"></i> New Template
            </button>
        </div>
        <div class="scrolling content">
            <div class="ui active inverted dimmer" v-if="loading">
                <div class="ui loader"></div>
            </div>
            <div class="ui cards">
                <div class="card" v-for="template in templates" :key="template.id">
                    <div class="content">
                        <div class="header">{{ template.name }}</div>
                        <div class="meta">
                            <span>Created: {{ new Date(template.created_at).toLocaleDateString() }}</span>
                        </div>
                        <div class="description">
                            <pre style="white-space: pre-wrap; font-family: inherit; margin: 0">{{ truncate(template.content, 150) }}</pre>
                        </div>
                    </div>
                    <div class="extra content">
                        <div class="ui two buttons">
                            <button class="ui yellow button" @click.stop="openEditModal(template)">
                                <i class="edit icon"></i> Edit
                            </button>
                            <button class="ui red button" @click.stop="deleteTemplate(template.id)">
                                <i class="trash icon"></i> Delete
                            </button>
                        </div>
                    </div>
                </div>
            </div>
            <div class="ui message" v-if="templates.length === 0 && !loading">
                No templates created yet. Create a template to use in campaigns.
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
    
    <!-- Template Form Modal -->
    <div class="ui large modal" id="modalCampaignTemplateForm">
        <i class="close icon"></i>
        <div class="header">{{ editingId ? 'Edit Template' : 'Create Template' }}</div>
        <div class="content">
            <form class="ui form">
                <div class="required field">
                    <label>Template Name</label>
                    <input v-model="form.name" type="text" placeholder="Welcome Message">
                </div>
                <div class="required field">
                    <label>Message Content</label>
                    <div class="ui segment">
                        <p><strong>Available Placeholders:</strong></p>
                        <div class="ui tiny buttons">
                            <button type="button" class="ui button" v-for="p in placeholders" :key="p" 
                                    @click="insertPlaceholder(p)">{{ p }}</button>
                        </div>
                    </div>
                    <textarea v-model="form.content" @input="updatePreview" rows="6" 
                              placeholder="Hello [NAME], welcome to our service!"></textarea>
                </div>
                <div class="field" v-if="previewText">
                    <label>Preview</label>
                    <div class="ui green segment">
                        <pre style="white-space: pre-wrap; font-family: inherit; margin: 0">{{ previewText }}</pre>
                    </div>
                </div>
            </form>
        </div>
        <div class="actions">
            <button class="ui positive button" :class="{loading: loading}" @click="submitForm">
                <i class="check icon"></i> Save Template
            </button>
        </div>
    </div>
    `
}
