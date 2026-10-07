<script lang="ts">
import type { WorkspaceSAMLConfig } from '@/api/workspace'
export function createSAMLForm(config?: WorkspaceSAMLConfig) {
  return { idp_entity_id: config?.idp_entity_id || '', sso_url: config?.sso_url || '', certificates: (config?.signing_certificates || []).join('\n'), metadata_xml: '', metadata_url: config?.metadata_url || '', metadata_source: config?.metadata_source || 'manual' as 'manual' | 'xml' | 'url', subject_attribute: config?.subject_attribute || '', allow_unspecified_name_id: config?.allow_unspecified_name_id || false, email_attribute: config?.email_attribute ?? 'email', name_attribute: config?.name_attribute ?? 'name', groups_attribute: config?.groups_attribute ?? 'groups' }
}
export function samlPayload(form: ReturnType<typeof createSAMLForm>): WorkspaceSAMLConfig {
  const certs = form.certificates.match(/-----BEGIN CERTIFICATE-----[\s\S]*?-----END CERTIFICATE-----/g) || []
  if (form.certificates.replace(/-----BEGIN CERTIFICATE-----[\s\S]*?-----END CERTIFICATE-----/g, '').trim()) throw new Error('Invalid certificates')
  if (form.metadata_source === 'manual' && (!form.idp_entity_id.trim() || !form.sso_url.trim() || !certs.length)) throw new Error('Missing manual configuration')
  if (form.metadata_source === 'xml' && !form.metadata_xml.trim() && !certs.length) throw new Error('Missing metadata')
  if (form.metadata_source === 'url' && !form.metadata_url.trim()) throw new Error('Missing metadata URL')
  return { idp_entity_id: form.idp_entity_id.trim(), sso_url: form.sso_url.trim(), signing_certificates: certs, metadata_source: form.metadata_source, ...(form.metadata_source === 'xml' && form.metadata_xml.trim() ? { metadata_xml: form.metadata_xml } : {}), ...(form.metadata_source === 'url' ? { metadata_url: form.metadata_url.trim() } : {}), subject_attribute: form.subject_attribute.trim(), allow_unspecified_name_id: form.allow_unspecified_name_id, email_attribute: form.email_attribute.trim(), name_attribute: form.name_attribute.trim(), groups_attribute: form.groups_attribute.trim(), authn_requests_signed: true }
}
</script>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
const model = defineModel<ReturnType<typeof createSAMLForm>>({ required: true })
const { t } = useI18n()
</script>
<template>
  <div class="saml-fields">
    <label><span>{{ t('workspace.identitySamlMetadataSource') }}</span><select v-model="model.metadata_source" class="input" name="metadata_source"><option value="manual">{{ t('workspace.identitySamlManual') }}</option><option value="xml">{{ t('workspace.identitySamlXML') }}</option><option value="url">{{ t('workspace.identitySamlURL') }}</option></select></label>
    <label v-if="model.metadata_source === 'xml'"><span>{{ t('workspace.identitySamlXML') }}</span><textarea v-model="model.metadata_xml" class="input" name="metadata_xml" rows="5" /><small>{{ t('workspace.identitySamlImportHint') }}</small></label>
    <label v-if="model.metadata_source === 'url'"><span>{{ t('workspace.identitySamlURL') }}</span><input v-model="model.metadata_url" class="input" name="metadata_url" type="url" required maxlength="2048"><small>{{ t('workspace.identitySamlImportHint') }}</small></label>
    <label><span>{{ t('workspace.identitySamlEntity') }}</span><input v-model="model.idp_entity_id" class="input" name="idp_entity_id" :required="model.metadata_source === 'manual'" maxlength="2048"></label>
    <label><span>{{ t('workspace.identitySamlSSOUrl') }}</span><input v-model="model.sso_url" class="input" name="sso_url" type="url" :required="model.metadata_source === 'manual'" maxlength="2048"></label>
    <label><span>{{ t('workspace.identitySamlCertificates') }}</span><textarea v-model="model.certificates" class="input" name="signing_certificates" rows="5" :required="model.metadata_source === 'manual'" /></label>
    <label><span>{{ t('workspace.identitySamlSubject') }}</span><input v-model="model.subject_attribute" class="input" name="subject_attribute" maxlength="200"><small>{{ t('workspace.identitySamlSubjectHint') }}</small></label>
    <label class="saml-check"><input v-model="model.allow_unspecified_name_id" type="checkbox" name="allow_unspecified_name_id"><span>{{ t('workspace.identitySamlUnspecified') }}</span></label>
    <label><span>{{ t('workspace.identitySamlEmail') }}</span><input v-model="model.email_attribute" class="input" name="email_attribute" required maxlength="200"></label>
    <label><span>{{ t('workspace.identitySamlName') }}</span><input v-model="model.name_attribute" class="input" name="name_attribute" maxlength="200"></label>
    <label><span>{{ t('workspace.identitySamlGroups') }}</span><input v-model="model.groups_attribute" class="input" name="groups_attribute" maxlength="200"></label>
    <p>{{ t('workspace.identitySamlDeploymentHint') }}</p>
  </div>
</template>
<style scoped>
.saml-fields { grid-column: 1 / -1; display: grid; min-width: 0; gap: 14px; }
label { display: grid; min-width: 0; gap: 6px; color: var(--color-text-secondary); font-size: 13px; }
.input { width: 100%; box-sizing: border-box; min-height: 40px; }
.saml-check { display: flex; align-items: flex-start; }
small, p { color: var(--color-text-secondary); font-size: 13px; line-height: 1.5; }
</style>
