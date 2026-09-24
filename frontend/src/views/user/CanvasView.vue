<template>
  <CanvasWorkspaceNav workspace>
    <main
      ref="workspaceRef"
      class="canvas-page"
      :class="{
        'canvas-page--drop-active': dropActive,
        'canvas-page--panel-open': leftPanelOpen || rightPanelOpen,
        'canvas-page--library-open': leftPanelOpen,
        'canvas-page--inspector-open': rightPanelOpen,
        [`canvas-page--grid-${gridStyle}`]: true,
      }"
      @dragenter.prevent="dropActive = true"
      @dragover.prevent
      @dragleave="handleDragLeave"
      @drop.prevent="handleCanvasDrop"
    >
      <VueFlow
        id="modurelay-image-canvas"
        v-model:nodes="flowNodes"
        v-model:edges="flowEdges"
        class="canvas-flow"
        :node-types="nodeTypes"
        :default-viewport="viewport"
        :default-edge-options="defaultEdgeOptions"
        :connection-line-options="connectionLineOptions"
        :is-valid-connection="validateConnection"
        :min-zoom="0.05"
        :max-zoom="5"
        :node-drag-threshold="4"
        :pane-click-distance="4"
        :pan-on-drag="effectiveInteractionMode === 'pan' ? true : [1]"
        :selection-on-drag="effectiveInteractionMode === 'select'"
        :delete-key-code="null"
        :connect-on-click="true"
        :edges-updatable="false"
        :elevate-edges-on-select="true"
        :elevate-nodes-on-select="true"
        @init="handleFlowInit"
        @nodes-initialized="handleNodesInitialized"
        @connect="handleConnect"
        @connect-start="handleConnectStart"
        @connect-end="handleConnectEnd"
        @node-click="handleNodeClick"
        @edge-click="handleEdgeClick"
        @pane-click="clearSelection"
        @node-drag-start="handleNodeDragStart"
        @node-drag="handleNodeDrag"
        @node-drag-stop="handleNodeDragStop"
        @selection-drag="handleNodeDrag"
        @selection-drag-stop="handleNodeDragStop"
        @node-context-menu="handleNodeContextMenu"
        @edge-context-menu="handleEdgeContextMenu"
        @pane-context-menu="handlePaneContextMenu"
        @viewport-change-end="handleViewportChange"
      />

      <header class="canvas-command-bar glass" :aria-label="t('canvas.commandBarAria')">
        <div class="canvas-command-bar__projects">
          <button
            type="button"
            class="canvas-icon-button"
            :class="{ 'is-active': leftPanelOpen }"
            :title="t('canvas.toggleLibrary')"
            :aria-label="t('canvas.toggleLibrary')"
            :aria-pressed="leftPanelOpen"
            @click="toggleLeftPanel"
          >
            <Icon name="menu" size="sm" />
          </button>
          <button
            type="button"
            class="canvas-brand-mark"
            :title="t('canvas.backToHome')"
            :aria-label="t('canvas.backToHome')"
            :disabled="projectActionLocked"
            @click="goToCanvasHome"
          >
            <Icon name="sparkles" size="sm" aria-hidden="true" />
          </button>
          <label class="sr-only" for="canvas-project-select">{{ t('canvas.project') }}</label>
          <select
            id="canvas-project-select"
            class="canvas-project-select"
            :value="currentProjectId || ''"
            :disabled="projectActionLocked"
            @change="switchProjectFromSelect"
          >
            <option v-for="project in store.projects" :key="project.id" :value="project.id">
              {{ project.title }}
            </option>
          </select>
        </div>

        <div class="canvas-command-bar__title">
          <label for="canvas-title" class="sr-only">{{ t('canvas.canvasTitleLabel') }}</label>
          <input
            id="canvas-title"
            v-model="title"
            class="canvas-title-input"
            maxlength="160"
            :disabled="!store.project"
            @input="markCanvasChanged"
          />
          <span class="canvas-save-state" :class="`canvas-save-state--${saveStatus}`" role="status" aria-live="polite">
            <span class="canvas-save-state__dot" aria-hidden="true"></span>
            {{ saveStateText }}
          </span>
        </div>

        <div class="canvas-command-bar__actions">
          <button
            type="button"
            class="canvas-icon-button"
            :title="t('canvas.newProject')"
            :aria-label="t('canvas.newProject')"
            :disabled="projectActionLocked"
            @click="createProject"
          >
            <Icon name="plus" size="sm" />
          </button>
          <button
            type="button"
            class="canvas-icon-button"
            :class="{ 'is-active': inspectorTab === 'history' && rightPanelOpen }"
            :title="t('canvas.history')"
            :aria-label="t('canvas.history')"
            :aria-pressed="inspectorTab === 'history' && rightPanelOpen"
            @click="openHistory"
          >
            <Icon name="clock" size="sm" />
          </button>
          <button
            type="button"
            class="canvas-icon-button canvas-mobile-panel-button"
            :class="{ 'is-active': inspectorTab === 'properties' && rightPanelOpen }"
            :title="t('canvas.properties')"
            :aria-label="t('canvas.properties')"
            :aria-pressed="inspectorTab === 'properties' && rightPanelOpen"
            @click="openProperties"
          >
            <Icon name="cog" size="sm" />
          </button>
          <button
            type="button"
            class="canvas-icon-button canvas-action-optional"
            :title="t('canvas.openInNewTab')"
            :aria-label="t('canvas.openInNewTab')"
            :disabled="!store.project"
            @click="openInNewTab"
          >
            <Icon name="externalLink" size="sm" />
          </button>
          <span class="canvas-command-divider" aria-hidden="true"></span>
          <button
            type="button"
            class="canvas-icon-button canvas-action-optional"
            :title="t('canvas.export')"
            :aria-label="t('canvas.export')"
            :disabled="!store.project"
            @click="exportProject"
          >
            <Icon name="download" size="sm" />
          </button>
          <button
            type="button"
            class="canvas-icon-button canvas-action-optional"
            :title="t('canvas.import')"
            :aria-label="t('canvas.import')"
            :disabled="!store.project || projectActionLocked"
            @click="importInput?.click()"
          >
            <Icon name="upload" size="sm" />
          </button>
          <button
            type="button"
            class="canvas-icon-button canvas-icon-button--danger canvas-action-optional"
            :title="t('canvas.deleteProject')"
            :aria-label="t('canvas.deleteProject')"
            :disabled="!store.project || projectActionLocked"
            @click="deleteProjectDialogOpen = true"
          >
            <Icon name="trash" size="sm" />
          </button>
          <button
            type="button"
            class="canvas-save-button"
            :disabled="!store.project || store.saving"
            :aria-busy="store.saving || undefined"
            @click="saveFromToolbar"
          >
            {{ store.saving ? t('canvas.saving') : t('canvas.save') }}
          </button>
        </div>
      </header>

      <aside
        class="canvas-node-library glass-panel"
        :class="{ 'is-open': leftPanelOpen }"
        :aria-label="t('canvas.nodesAria')"
      >
        <nav class="canvas-library-tabs" role="tablist" :aria-label="t('canvas.library')">
          <button
            v-for="tab in libraryTabs"
            :key="tab.id"
            type="button"
            role="tab"
            :aria-selected="libraryTab === tab.id"
            :class="{ 'is-active': libraryTab === tab.id }"
            @click="libraryTab = tab.id"
          >
            {{ tab.label }}
          </button>
          <button
            type="button"
            class="canvas-panel-close"
            :aria-label="t('common.close')"
            @click="leftPanelOpen = false"
          >
            <Icon name="x" size="sm" />
          </button>
        </nav>

        <div class="canvas-library-summary">
          <span v-if="libraryTab === 'nodes'">{{ t('canvas.nodeCountCompact', { count: nodes.length }) }}</span>
          <span v-else-if="libraryTab === 'assets'">{{ t('canvas.assetCount', { count: canvasAssets.length }) }}</span>
          <span v-else>{{ t('canvas.promptCount', { count: promptLibrary.length }) }}</span>
        </div>

        <label class="canvas-library-search">
          <span class="sr-only">{{ t('canvas.searchLibrary') }}</span>
          <Icon name="search" size="sm" aria-hidden="true" />
          <input v-model="librarySearch" type="search" :placeholder="librarySearchPlaceholder" />
        </label>

        <div v-if="libraryTab === 'nodes'" class="canvas-node-library__list" role="tabpanel">
          <button
            v-for="item in filteredPalette"
            :key="item.type"
            type="button"
            class="canvas-node-library__item"
            :data-node-type="item.type"
            :disabled="nodes.length >= maxNodes"
            @click="addNode(item.type)"
          >
            <span class="canvas-node-library__icon" aria-hidden="true">
              <Icon :name="item.icon" size="sm" />
            </span>
            <span>
              <strong>{{ item.label }}</strong>
              <small>{{ item.description }}</small>
            </span>
            <Icon name="plus" size="xs" aria-hidden="true" />
          </button>
          <div v-if="!filteredPalette.length" class="canvas-library-empty">{{ t('canvas.noSearchResults') }}</div>
        </div>

        <div v-else-if="libraryTab === 'assets'" class="canvas-library-scroll" role="tabpanel">
          <button
            v-for="asset in filteredAssets"
            :key="asset.id"
            type="button"
            class="canvas-asset-item"
            @click="focusNode(asset.id)"
          >
            <span class="canvas-asset-item__preview">
              <img v-if="asset.data.url" :src="String(asset.data.url)" :alt="String(asset.data.label)" />
              <Icon v-else name="grid" size="sm" aria-hidden="true" />
            </span>
            <span>
              <strong>{{ asset.data.label }}</strong>
              <small>{{ nodeKindLabel(asset.type) }}</small>
            </span>
          </button>
          <div v-if="!filteredAssets.length" class="canvas-library-empty">{{ t('canvas.noAssets') }}</div>
        </div>

        <div v-else class="canvas-library-scroll" role="tabpanel">
          <button
            v-for="prompt in filteredPrompts"
            :key="prompt.id"
            type="button"
            class="canvas-prompt-item"
            @click="addPromptFromLibrary(prompt)"
          >
            <span class="canvas-node-library__icon" aria-hidden="true"><Icon name="document" size="sm" /></span>
            <span>
              <strong>{{ prompt.label }}</strong>
              <small>{{ prompt.prompt }}</small>
              <em>{{ prompt.projectTitle }}</em>
            </span>
            <Icon name="plus" size="xs" aria-hidden="true" />
          </button>
          <div v-if="!filteredPrompts.length" class="canvas-library-empty">{{ t('canvas.noPrompts') }}</div>
        </div>

        <div class="canvas-node-library__footer">
          <span>{{ t('canvas.nodeCount', { count: nodes.length, max: maxNodes }) }}</span>
          <span>{{ t('canvas.edgeCount', { count: edges.length }) }}</span>
        </div>
      </aside>

      <aside
        class="canvas-inspector"
        :class="{ 'is-open': rightPanelOpen }"
        :aria-label="inspectorTab === 'history' ? t('canvas.history') : t('canvas.propertiesAria')"
      >
        <div class="canvas-inspector__tabs glass-panel" role="tablist" :aria-label="t('canvas.inspector')">
          <button
            type="button"
            role="tab"
            :aria-selected="inspectorTab === 'properties'"
            :class="{ 'is-active': inspectorTab === 'properties' }"
            @click="inspectorTab = 'properties'"
          >
            {{ t('canvas.properties') }}
          </button>
          <button
            type="button"
            role="tab"
            :aria-selected="inspectorTab === 'history'"
            :class="{ 'is-active': inspectorTab === 'history' }"
            @click="selectHistoryTab"
          >
            {{ t('canvas.history') }}
          </button>
          <button type="button" class="canvas-panel-close" :aria-label="t('common.close')" @click="rightPanelOpen = false">
            <Icon name="x" size="sm" />
          </button>
        </div>

        <section v-if="inspectorTab === 'properties'" class="canvas-inspector__body surface-panel" role="tabpanel">
          <template v-if="selectedNode">
            <div class="canvas-inspector__identity">
              <span class="canvas-node-library__icon" aria-hidden="true">
                <Icon :name="selectedNodeIcon" size="sm" />
              </span>
              <div>
                <span>{{ selectedNodeKind }}</span>
                <strong>{{ selectedNode.data.label }}</strong>
              </div>
            </div>

            <label class="canvas-field" for="canvas-node-label">
              <span>{{ t('canvas.label') }}</span>
              <input id="canvas-node-label" v-model="selectedNode.data.label" maxlength="160" @input="markCanvasChanged" />
            </label>

            <label v-if="selectedNode.type === 'prompt'" class="canvas-field" for="canvas-prompt">
              <span>{{ t('canvas.prompt') }}</span>
              <textarea id="canvas-prompt" v-model="selectedNode.data.prompt" rows="9" maxlength="32000" @input="markCanvasChanged"></textarea>
              <small>{{ String(selectedNode.data.prompt || '').length }} / 32000</small>
            </label>

            <template v-else-if="selectedNode.type === 'text'">
              <label class="canvas-field" for="canvas-text-content">
                <span>{{ t('canvas.textContent') }}</span>
                <textarea id="canvas-text-content" v-model="selectedNode.data.content" rows="10" maxlength="100000" @input="markCanvasChanged"></textarea>
                <small>{{ String(selectedNode.data.content || '').length }} / 100000</small>
              </label>
              <label class="canvas-field" for="canvas-text-size">
                <span>{{ t('canvas.fontSize') }} <output>{{ selectedNode.data.fontSize || 16 }}px</output></span>
                <input id="canvas-text-size" v-model.number="selectedNode.data.fontSize" type="range" min="10" max="72" step="1" @input="markCanvasChanged" />
              </label>
            </template>

            <template v-else-if="selectedNode.type === 'reference'">
              <div class="canvas-upload-preview">
                <img v-if="selectedNode.data.url" :src="String(selectedNode.data.url)" :alt="String(selectedNode.data.fileName || selectedNode.data.label)" />
                <div v-else>
                  <Icon name="upload" size="lg" aria-hidden="true" />
                  <span>{{ t('canvas.referenceEmpty') }}</span>
                </div>
              </div>
              <button
                type="button"
                class="canvas-secondary-button canvas-button-full"
                :disabled="uploadingNodeId === selectedNode.id"
                :aria-busy="uploadingNodeId === selectedNode.id || undefined"
                @click="referenceInput?.click()"
              >
                <Icon name="upload" size="sm" aria-hidden="true" />
                {{ uploadingNodeId === selectedNode.id ? t('canvas.uploading') : t('canvas.uploadReference') }}
              </button>
              <p class="canvas-field-help">{{ t('canvas.referenceRequirements') }}</p>
            </template>

            <template v-else-if="selectedNode.type === 'generation'">
              <label class="canvas-field" for="canvas-model">
                <span>{{ t('canvas.model') }}</span>
                <select id="canvas-model" v-model="selectedNode.data.model" @change="markCanvasChanged">
                  <option value="gpt-image-1">gpt-image-1</option>
                  <option value="dall-e-3">dall-e-3</option>
                </select>
              </label>

              <div class="canvas-inspector__section">
                <h3>{{ t('canvas.connections') }}</h3>
                <label class="canvas-field" for="canvas-prompt-source">
                  <span>{{ t('canvas.promptSource') }}</span>
                  <select id="canvas-prompt-source" :value="connectedPrompt?.id || ''" @change="setGenerationInput('prompt', $event)">
                    <option value="">{{ t('canvas.notConnected') }}</option>
                    <option v-for="node in promptNodes" :key="node.id" :value="node.id">{{ node.data.label }}</option>
                  </select>
                </label>
                <label class="canvas-field" for="canvas-reference-source">
                  <span>{{ t('canvas.referenceSource') }}</span>
                  <select id="canvas-reference-source" :value="connectedReference?.id || ''" @change="setGenerationInput('reference', $event)">
                    <option value="">{{ t('canvas.none') }}</option>
                    <option v-for="node in referenceNodes" :key="node.id" :value="node.id">{{ node.data.label }}</option>
                  </select>
                </label>
              </div>

              <label class="canvas-field" for="canvas-runtime-key">
                <span>{{ t('canvas.runtimeApiKey') }}</span>
                <span class="canvas-secret-input">
                  <input
                    id="canvas-runtime-key"
                    v-model="runtimeApiKey"
                    :type="showRuntimeApiKey ? 'text' : 'password'"
                    autocomplete="off"
                    spellcheck="false"
                    :placeholder="t('canvas.runtimeApiKeyPlaceholder')"
                  />
                  <button
                    type="button"
                    :aria-label="showRuntimeApiKey ? t('canvas.hideApiKey') : t('canvas.showApiKey')"
                    :title="showRuntimeApiKey ? t('canvas.hideApiKey') : t('canvas.showApiKey')"
                    @click="showRuntimeApiKey = !showRuntimeApiKey"
                  >
                    <Icon :name="showRuntimeApiKey ? 'eyeOff' : 'eye'" size="sm" />
                  </button>
                </span>
                <small>{{ t('canvas.runtimeApiKeyHelp') }}</small>
              </label>

              <button
                type="button"
                class="canvas-primary-button canvas-button-full"
                :disabled="generatingNodeId !== null || !runtimeApiKey || !connectedPrompt"
                :aria-busy="generatingNodeId === selectedNode.id || undefined"
                @click="generateFromNode"
              >
                <Icon name="sparkles" size="sm" aria-hidden="true" />
                {{ generatingNodeId === selectedNode.id ? t('canvas.generating') : t('canvas.generateImage') }}
              </button>
              <p v-if="selectedNode.data.error" class="canvas-inline-error" role="alert">{{ selectedNode.data.error }}</p>
            </template>

            <template v-else-if="selectedNode.type === 'image'">
              <div class="canvas-upload-preview canvas-upload-preview--output">
                <img v-if="selectedNode.data.url" :src="String(selectedNode.data.url)" :alt="String(selectedNode.data.label)" />
                <div v-else>
                  <Icon name="grid" size="lg" aria-hidden="true" />
                  <span>{{ t('canvas.imageOutput') }}</span>
                </div>
              </div>
              <label class="canvas-field" for="canvas-generation-source">
                <span>{{ t('canvas.generationSource') }}</span>
                <select id="canvas-generation-source" :value="connectedGeneration?.id || ''" @change="setImageInput">
                  <option value="">{{ t('canvas.notConnected') }}</option>
                  <option v-for="node in generationNodes" :key="node.id" :value="node.id">{{ node.data.label }}</option>
                </select>
              </label>
              <button
                v-if="selectedNode.data.url"
                type="button"
                class="canvas-secondary-button canvas-button-full"
                @click="downloadSelectedImage"
              >
                <Icon name="download" size="sm" aria-hidden="true" />
                {{ t('canvas.downloadImage') }}
              </button>
            </template>

            <template v-else-if="selectedNode.type === 'video' || selectedNode.type === 'audio'">
              <div class="canvas-upload-preview" :class="{ 'canvas-upload-preview--audio': selectedNode.type === 'audio' }">
                <video v-if="selectedNode.type === 'video' && selectedNode.data.url" :src="String(selectedNode.data.url)" controls playsinline preload="metadata"></video>
                <audio v-else-if="selectedNode.type === 'audio' && selectedNode.data.url" :src="String(selectedNode.data.url)" controls preload="metadata"></audio>
                <div v-else>
                  <Icon :name="selectedNode.type === 'audio' ? 'music' : 'play'" size="lg" aria-hidden="true" />
                  <span>{{ selectedNode.type === 'audio' ? t('canvas.audioEmpty') : t('canvas.videoEmpty') }}</span>
                </div>
              </div>
              <button type="button" class="canvas-secondary-button canvas-button-full" :disabled="uploadingNodeId === selectedNode.id" @click="mediaUploadTargetId = selectedNode.id; referenceInput?.click()">
                <Icon name="upload" size="sm" aria-hidden="true" />{{ uploadingNodeId === selectedNode.id ? t('canvas.uploading') : t('canvas.uploadMedia') }}
              </button>
              <button v-if="selectedNode.data.url" type="button" class="canvas-secondary-button canvas-button-full" @click="downloadSelectedImage">
                <Icon name="download" size="sm" aria-hidden="true" />{{ t('canvas.downloadMedia') }}
              </button>
            </template>

            <template v-else-if="selectedNode.type === 'config'">
              <label class="canvas-field" for="canvas-generation-mode">
                <span>{{ t('canvas.generationMode') }}</span>
                <select id="canvas-generation-mode" v-model="selectedNode.data.generationMode" @change="handleConfigModeChange">
                  <option value="image">{{ t('canvas.generationModes.image') }}</option>
                  <option value="video">{{ t('canvas.generationModes.video') }}</option>
                  <option value="audio">{{ t('canvas.generationModes.audio') }}</option>
                  <option value="text">{{ t('canvas.generationModes.text') }}</option>
                </select>
              </label>
              <label class="canvas-field" for="canvas-config-model"><span>{{ t('canvas.model') }}</span><input id="canvas-config-model" v-model="selectedNode.data.model" maxlength="160" :readonly="selectedNode.data.generationMode === 'audio'" @input="markCanvasChanged" /><small v-if="selectedNode.data.generationMode === 'audio'">{{ t('canvas.grokTtsHelp') }}</small></label>
              <label class="canvas-field" for="canvas-config-prompt"><span>{{ t('canvas.prompt') }}</span><textarea id="canvas-config-prompt" v-model="selectedNode.data.prompt" rows="6" maxlength="32000" @input="markCanvasChanged"></textarea></label>
              <div v-if="selectedNode.data.generationMode === 'image'" class="canvas-option-grid">
                <label class="canvas-field"><span>{{ t('canvas.size') }}</span><select v-model="selectedNode.data.size" @change="markCanvasChanged"><option>1024x1024</option><option>1536x1024</option><option>1024x1536</option></select></label>
                <label class="canvas-field"><span>{{ t('canvas.count') }}</span><input v-model.number="selectedNode.data.count" type="number" min="1" max="4" @input="markCanvasChanged" /></label>
              </div>
              <div v-else-if="selectedNode.data.generationMode === 'video'" class="canvas-option-grid">
                <label class="canvas-field"><span>{{ t('canvas.aspectRatio') }}</span><select v-model="selectedNode.data.aspectRatio" @change="markCanvasChanged"><option>16:9</option><option>9:16</option><option>1:1</option></select></label>
                <label class="canvas-field"><span>{{ t('canvas.duration') }}</span><select v-model.number="selectedNode.data.seconds" @change="markCanvasChanged"><option :value="6">6s</option><option :value="8">8s</option><option :value="10">10s</option></select></label>
                <label class="canvas-field"><span>{{ t('canvas.resolution') }}</span><select v-model="selectedNode.data.resolution" @change="markCanvasChanged"><option>480p</option><option>720p</option><option>1080p</option></select></label>
              </div>
              <div v-else-if="selectedNode.data.generationMode === 'audio'" class="canvas-option-grid">
                <label class="canvas-field"><span>{{ t('canvas.voice') }}</span><select v-model="selectedNode.data.voice" @change="markCanvasChanged"><option v-for="voice in CANVAS_GROK_TTS_VOICES" :key="voice" :value="voice">{{ voice }}</option></select></label>
                <label class="canvas-field"><span>{{ t('canvas.language') }}</span><input v-model.trim="selectedNode.data.language" maxlength="16" placeholder="en" @input="markCanvasChanged" /><small>{{ t('canvas.languageHelp') }}</small></label>
              </div>
              <label class="canvas-field" for="canvas-config-runtime-key"><span>{{ t('canvas.runtimeApiKey') }}</span><input id="canvas-config-runtime-key" v-model="runtimeApiKey" type="password" autocomplete="off" spellcheck="false" :placeholder="t('canvas.runtimeApiKeyPlaceholder')" /><small>{{ t('canvas.runtimeApiKeyHelp') }}</small></label>
              <button type="button" class="canvas-primary-button canvas-button-full" :disabled="generatingNodeId !== null || !runtimeApiKey || !selectedConfigHasPrompt" @click="generateFromNode">
                <Icon name="sparkles" size="sm" aria-hidden="true" />{{ generatingNodeId === selectedNode.id ? t('canvas.generating') : t(`canvas.generateActions.${selectedNode.data.generationMode || 'image'}`) }}
              </button>
              <p v-if="selectedNode.data.error" class="canvas-inline-error" role="alert">{{ selectedNode.data.error }}</p>
            </template>

            <template v-else-if="selectedNode.type === 'group'">
              <p class="canvas-field-help">{{ t('canvas.groupHint') }}</p>
              <button type="button" class="canvas-secondary-button canvas-button-full" @click="ungroupSelection">
                <Icon name="group" size="sm" aria-hidden="true" />{{ t('canvas.ungroup') }}
              </button>
            </template>

            <div class="canvas-danger-zone">
              <button type="button" class="canvas-danger-button canvas-button-full" @click="deleteSelected">
                <Icon name="trash" size="sm" aria-hidden="true" />
                {{ t('canvas.deleteNode') }}
              </button>
            </div>
          </template>

          <template v-else-if="selectedEdge">
            <div class="canvas-inspector__identity">
              <span class="canvas-node-library__icon" aria-hidden="true"><Icon name="link" size="sm" /></span>
              <div>
                <span>{{ t('canvas.connection') }}</span>
                <strong>{{ edgeSourceLabel }} → {{ edgeTargetLabel }}</strong>
              </div>
            </div>
            <dl class="canvas-connection-summary">
              <div><dt>{{ t('canvas.from') }}</dt><dd>{{ edgeSourceLabel }}</dd></div>
              <div><dt>{{ t('canvas.to') }}</dt><dd>{{ edgeTargetLabel }}</dd></div>
            </dl>
            <button type="button" class="canvas-danger-button canvas-button-full" @click="deleteSelected">
              <Icon name="trash" size="sm" aria-hidden="true" />
              {{ t('canvas.deleteConnection') }}
            </button>
          </template>

          <div v-else class="canvas-project-overview">
            <span class="canvas-panel-eyebrow">{{ t('canvas.projectOverview') }}</span>
            <h2>{{ title }}</h2>
            <dl>
              <div><dt>{{ t('canvas.nodes') }}</dt><dd>{{ nodes.length }}</dd></div>
              <div><dt>{{ t('canvas.connections') }}</dt><dd>{{ edges.length }}</dd></div>
              <div><dt>{{ t('canvas.revision') }}</dt><dd>{{ store.project?.revision || 0 }}</dd></div>
            </dl>
          </div>
        </section>

        <section v-else class="canvas-inspector__body surface-panel" role="tabpanel">
          <div class="canvas-history-header">
            <div>
              <span class="canvas-panel-eyebrow">{{ t('canvas.versionHistory') }}</span>
              <h2>{{ t('canvas.checkpoints') }}</h2>
            </div>
            <button
              type="button"
              class="canvas-secondary-button"
              :disabled="checkpointing || !store.project || projectActionLocked"
              :aria-busy="checkpointing || undefined"
              @click="createCheckpoint"
            >
              <Icon name="plus" size="sm" aria-hidden="true" />
              {{ t('canvas.checkpoint') }}
            </button>
          </div>
          <div v-if="revisionsLoading" class="canvas-history-state" role="status">{{ t('common.loading') }}</div>
          <div v-else-if="!revisions.length" class="canvas-history-state">{{ t('canvas.noCheckpoints') }}</div>
          <ol v-else class="canvas-history-list">
            <li v-for="revision in revisions" :key="revision.id">
              <div>
                <strong>{{ t('canvas.revisionLabel', { revision: revision.revision }) }}</strong>
                <span>{{ formatTimestamp(revision.created_at) }}</span>
              </div>
              <button type="button" class="canvas-text-button" :disabled="projectActionLocked" @click="restoreCandidate = revision">
                {{ t('canvas.restore') }}
              </button>
            </li>
          </ol>
        </section>
      </aside>

      <div class="canvas-tool-dock glass" role="toolbar" :aria-label="t('canvas.toolsAria')">
        <button
          type="button"
          :class="{ 'is-active': interactionMode === 'pan' }"
          :title="t('canvas.panTool')"
          :aria-label="t('canvas.panTool')"
          :aria-pressed="interactionMode === 'pan'"
          @click="interactionMode = 'pan'"
        >
          <Icon name="hand" size="sm" />
        </button>
        <button
          type="button"
          :class="{ 'is-active': interactionMode === 'select' }"
          :title="t('canvas.selectTool')"
          :aria-label="t('canvas.selectTool')"
          :aria-pressed="interactionMode === 'select'"
          @click="interactionMode = 'select'"
        >
          <Icon name="cursorArrow" size="sm" />
        </button>
        <span aria-hidden="true"></span>
        <button type="button" :disabled="!canUndo" :title="t('canvas.undo')" :aria-label="t('canvas.undo')" @click="undoCanvas">
          <Icon name="undo" size="sm" />
        </button>
        <button type="button" :disabled="!canRedo" :title="t('canvas.redo')" :aria-label="t('canvas.redo')" @click="redoCanvas">
          <Icon name="redo" size="sm" />
        </button>
        <span aria-hidden="true"></span>
        <button
          v-for="item in palette"
          :key="`dock-${item.type}`"
          type="button"
          :data-node-type="item.type"
          :disabled="nodes.length >= maxNodes"
          :title="item.label"
          :aria-label="item.label"
          @click="addNode(item.type)"
        >
          <Icon :name="item.icon" size="sm" />
        </button>
        <span aria-hidden="true"></span>
        <button
          type="button"
          :class="{ 'is-active': preferencesOpen }"
          :title="t('canvas.appearance')"
          :aria-label="t('canvas.appearance')"
          :aria-expanded="preferencesOpen"
          @click="preferencesOpen = !preferencesOpen"
        >
          <Icon name="grid" size="sm" />
        </button>
      </div>

      <div v-if="selectedNodes.length > 1" class="canvas-selection-tools glass-popover" role="toolbar" :aria-label="t('canvas.selectionToolbar')">
        <span>{{ t('canvas.selectedCount', { count: selectedNodes.length }) }}</span>
        <button v-if="canGroupSelection" type="button" @click="groupSelection"><Icon name="group" size="sm" aria-hidden="true" />{{ t('canvas.group') }}</button>
        <button v-if="canUngroupSelection" type="button" @click="ungroupSelection"><Icon name="group" size="sm" aria-hidden="true" />{{ t('canvas.ungroup') }}</button>
      </div>

      <section v-if="preferencesOpen" class="canvas-appearance-popover glass-popover" :aria-label="t('canvas.appearance')">
        <div>
          <strong>{{ t('canvas.appearance') }}</strong>
          <button type="button" :aria-label="t('common.close')" @click="preferencesOpen = false"><Icon name="x" size="sm" /></button>
        </div>
        <span>{{ t('canvas.gridStyle') }}</span>
        <div class="canvas-grid-options">
          <button
            v-for="option in gridOptions"
            :key="option.id"
            type="button"
            :class="{ 'is-active': gridStyle === option.id }"
            :aria-pressed="gridStyle === option.id"
            @click="setGridStyle(option.id)"
          >
            {{ option.label }}
          </button>
        </div>
      </section>

      <div class="canvas-zoom-controls glass" :aria-label="t('canvas.zoomControlsAria')">
        <button type="button" :class="{ 'is-active': miniMapOpen }" :title="t('canvas.toggleMinimap')" :aria-label="t('canvas.toggleMinimap')" :aria-pressed="miniMapOpen" @click="miniMapOpen = !miniMapOpen"><Icon name="compass" size="sm" /></button>
        <button type="button" :title="t('canvas.fitView')" :aria-label="t('canvas.fitView')" @click="fitCanvas">
          <Icon name="fitScreen" size="sm" />
        </button>
        <button type="button" :title="t('canvas.zoomOut')" :aria-label="t('canvas.zoomOut')" @click="zoomCanvasOut">
          <Icon name="minus" size="sm" />
        </button>
        <input
          class="canvas-zoom-range"
          type="range"
          min="5"
          max="500"
          step="5"
          :value="zoomPercent"
          :aria-label="t('canvas.zoomSlider')"
          @input="setZoomFromRange"
        />
        <button type="button" :title="t('canvas.zoomIn')" :aria-label="t('canvas.zoomIn')" @click="zoomCanvasIn">
          <Icon name="plus" size="sm" />
        </button>
        <button type="button" class="canvas-zoom-value" :title="t('canvas.resetZoom')" @click="resetViewport">
          {{ zoomPercent }}%
        </button>
      </div>

      <CanvasMiniMap v-if="miniMapOpen && nodes.length" :nodes="nodes" :viewport="viewport" :viewport-size="viewportSize" @viewport-change="setViewportFromMiniMap" />

      <div v-if="contextMenu" class="canvas-context-menu glass-popover" :style="{ left: `${Math.min(contextMenu.x, viewportSize.width - 210)}px`, top: `${Math.min(contextMenu.y, viewportSize.height - 320)}px` }" role="menu" data-canvas-no-zoom @pointerdown.stop>
        <template v-if="contextMenu.type === 'pane' || contextMenu.type === 'connection'">
          <span v-if="contextMenu.type === 'connection'" class="canvas-context-menu__label">{{ t('canvas.createConnectedNode') }}</span>
          <button v-for="item in contextActionPalette" :key="`context-${item.type}`" type="button" role="menuitem" @click="runContextAction(item.type)"><Icon :name="item.icon" size="sm" aria-hidden="true" />{{ item.label }}</button>
        </template>
        <template v-else>
          <template v-if="contextMenu.type === 'node' && (contextMenuNode?.type === 'image' || contextMenuNode?.type === 'reference') && contextMenuNode.data.url">
            <button type="button" role="menuitem" @click="runContextAction('edit-image')"><Icon name="edit" size="sm" aria-hidden="true" />{{ t('canvas.editImage') }}</button>
            <button type="button" role="menuitem" @click="runContextAction('mask-image')"><Icon name="focus" size="sm" aria-hidden="true" />{{ t('canvas.editors.maskTitle') }}</button>
            <button type="button" role="menuitem" @click="runContextAction('angle-image')"><Icon name="compass" size="sm" aria-hidden="true" />{{ t('canvas.editors.angleTitle') }}</button>
            <span class="canvas-context-menu__divider" aria-hidden="true"></span>
          </template>
          <template v-if="contextMenu.type === 'node' && contextMenuNode?.type === 'video' && contextMenuNode.data.url">
            <button type="button" role="menuitem" @click="runContextAction('frame-first')"><Icon name="image" size="sm" aria-hidden="true" />{{ t('canvas.videoFrames.first') }}</button>
            <button type="button" role="menuitem" @click="runContextAction('frame-current')"><Icon name="image" size="sm" aria-hidden="true" />{{ t('canvas.videoFrames.current') }}</button>
            <button type="button" role="menuitem" @click="runContextAction('frame-last')"><Icon name="image" size="sm" aria-hidden="true" />{{ t('canvas.videoFrames.last') }}</button>
            <span class="canvas-context-menu__divider" aria-hidden="true"></span>
          </template>
          <button v-if="contextMenu.type === 'node'" type="button" role="menuitem" @click="runContextAction('duplicate')"><Icon name="copy" size="sm" aria-hidden="true" />{{ t('canvas.duplicate') }}</button>
          <button v-if="contextMenu.type === 'node' && canGroupSelection" type="button" role="menuitem" @click="runContextAction('group')"><Icon name="group" size="sm" aria-hidden="true" />{{ t('canvas.group') }}</button>
          <button v-if="contextMenu.type === 'node' && canUngroupSelection" type="button" role="menuitem" @click="runContextAction('ungroup')"><Icon name="group" size="sm" aria-hidden="true" />{{ t('canvas.ungroup') }}</button>
          <button type="button" class="is-danger" role="menuitem" @click="runContextAction('delete')"><Icon name="trash" size="sm" aria-hidden="true" />{{ contextMenu.type === 'edge' ? t('canvas.deleteConnection') : t('canvas.deleteNode') }}</button>
        </template>
      </div>

      <div v-if="!initializing && !nodes.length" class="canvas-empty-state">
        <span class="canvas-empty-state__icon" aria-hidden="true"><Icon name="sparkles" size="lg" /></span>
        <h1>{{ t('canvas.emptyTitle') }}</h1>
        <p>{{ t('canvas.emptyDescription') }}</p>
        <div>
          <button type="button" class="canvas-primary-button" @click="createStarterFlow">
            <Icon name="sparkles" size="sm" aria-hidden="true" />
            {{ t('canvas.createWorkflow') }}
          </button>
          <button type="button" class="canvas-secondary-button" @click="addNode('reference')">
            <Icon name="upload" size="sm" aria-hidden="true" />
            {{ t('canvas.addReference') }}
          </button>
        </div>
      </div>

      <div v-if="initializing" class="canvas-loading" role="status">
        <span class="canvas-loading__spinner" aria-hidden="true"></span>
        {{ t('canvas.loadingWorkspace') }}
      </div>

      <div v-if="dropActive" class="canvas-drop-overlay" aria-hidden="true">
        <Icon name="upload" size="xl" />
        <strong>{{ t('canvas.dropToUpload') }}</strong>
      </div>

      <div
        v-if="visibleMessage"
        class="canvas-message-banner glass-popover"
        :class="`canvas-message-banner--${messageToneResolved}`"
        :role="messageToneResolved === 'error' ? 'alert' : 'status'"
      >
        <Icon v-if="messageToneResolved === 'success'" name="checkCircle" size="sm" aria-hidden="true" />
        <Icon v-else name="exclamationTriangle" size="sm" aria-hidden="true" />
        <span>{{ visibleMessage }}</span>
        <button v-if="store.error" type="button" @click="reloadCurrentProject">{{ t('canvas.reload') }}</button>
        <button type="button" :aria-label="t('common.close')" @click="dismissError"><Icon name="x" size="sm" /></button>
      </div>

      <input ref="importInput" type="file" accept="application/json,.json,.canvas" hidden @change="importProject" />
      <input ref="referenceInput" type="file" accept="image/png,image/jpeg,image/webp,image/gif,video/mp4,video/webm,audio/mpeg,audio/wav,audio/ogg,audio/opus,audio/mp4,audio/aac,audio/flac" hidden @change="uploadSelectedReference" />

      <CanvasImageEditorDialog
        :show="Boolean(imageEditorNode)"
        :source="String(imageEditorNode?.data.url || '')"
        :processing="imageEditorProcessing"
        @close="closeImageEditor"
        @apply="applyImageEdit"
      />
      <CanvasImageMaskDialog
        :show="Boolean(maskEditorNode)"
        :source="String(maskEditorNode?.data.url || '')"
        :processing="maskEditorProcessing"
        @close="closeMaskEditor"
        @confirm="applyImageMask"
      />
      <CanvasImageAngleDialog
        :show="Boolean(angleEditorNode)"
        :source="String(angleEditorNode?.data.url || '')"
        :processing="angleEditorProcessing"
        @close="closeAngleEditor"
        @confirm="applyImageAngle"
      />

      <ConfirmDialog
        :show="deleteProjectDialogOpen"
        :title="t('canvas.deleteProject')"
        :message="t('canvas.deleteProjectConfirm', { title })"
        :confirm-text="t('common.delete')"
        :confirming="busyProjectAction"
        danger
        @confirm="confirmDeleteProject"
        @cancel="deleteProjectDialogOpen = false"
      />
      <ConfirmDialog
        :show="Boolean(restoreCandidate)"
        :title="t('canvas.restoreCheckpoint')"
        :message="t('canvas.restoreCheckpointConfirm', { revision: restoreCandidate?.revision || 0 })"
        :confirm-text="t('canvas.restore')"
        :confirming="restoring"
        @confirm="confirmRestore"
        @cancel="restoreCandidate = null"
      />
    </main>
  </CanvasWorkspaceNav>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  ConnectionLineType,
  MarkerType,
  VueFlow,
  type Connection,
  type EdgeMouseEvent,
  type NodeDragEvent,
  type NodeMouseEvent,
  type ValidConnectionFunc,
  type ViewportTransform,
  type VueFlowStore,
} from '@vue-flow/core'
import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'
import CanvasWorkspaceNav from '@/components/canvas/CanvasWorkspaceNav.vue'
import CanvasNodeRenderer from '@/components/canvas/CanvasNode.vue'
import CanvasMiniMap from '@/components/canvas/CanvasMiniMap.vue'
import CanvasImageEditorDialog from '@/components/canvas/CanvasImageEditorDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useCanvasStore } from '@/stores/canvas'
import { getCanvasDraft, removeCanvasDraft, saveCanvasDraft } from '@/repositories/canvasDrafts'
import {
  CANVAS_GROK_TTS_MODEL,
  CANVAS_GROK_TTS_VOICES,
  canvasAPI,
  type CanvasDocument,
  type CanvasAsset,
  type ImageTask,
  type CanvasNode,
  type CanvasNodeData,
  type CanvasNodeType,
  type CanvasRevision,
} from '@/api/canvas'
import { videoAPI, type VideoGenerationTask } from '@/api/video'
import { editCanvasImage, type CanvasImageEditOperation, type CanvasImageEditResult } from '@/utils/canvasImageEditor'
import { readCanvasClipboard } from '@/utils/canvasClipboard'
import { captureCanvasVideoFrame, type CanvasVideoFramePosition } from '@/utils/canvasVideoFrame'
import { buildCanvasImageAnglePrompt, normalizeCanvasImageAngle, type CanvasImageAngleParams } from '@/utils/canvasImageAngle'
import type { CanvasImageMaskPayload } from '@/utils/canvasImageMask'
import {
  CANVAS_HANDLES,
  CANVAS_NODE_DEFAULT_SIZE,
  applyCanvasGroupSelection,
  applyCanvasUngroupSelection,
  canGroupCanvasNodes,
  canUngroupCanvasNodes,
  collectCanvasGroupMembers,
  connectedNode,
  createCanvasEdge,
  findCanvasGroupDropTarget,
  findContainingCanvasGroupId,
  findAvailableCanvasPosition,
  getCanvasGroupWrapRect,
  getConnectionRule,
  isCanvasConnectionValid,
  normalizeCanvasDocument,
  serializeCanvasDocument,
  snapCanvasNodesIntoGroup,
  type CanvasFlowEdge,
  type CanvasFlowNode,
} from '@/utils/canvasGraph'

const CanvasImageMaskDialog = defineAsyncComponent(() => import('@/components/canvas/CanvasImageMaskDialog.vue'))
const CanvasImageAngleDialog = defineAsyncComponent(() => import('@/components/canvas/CanvasImageAngleDialog.vue'))

type InspectorTab = 'properties' | 'history'
type SaveStatus = 'idle' | 'dirty' | 'saving' | 'saved' | 'error'
type PaletteIcon = 'document' | 'link' | 'sparkles' | 'image' | 'type' | 'play' | 'music' | 'cog' | 'group'
type LibraryTab = 'nodes' | 'assets' | 'prompts'
type InteractionMode = 'pan' | 'select'
type GridStyle = 'dots' | 'lines' | 'blank'
type LocalHistorySnapshot = { title: string; document: CanvasDocument; signature: string }
type PromptLibraryItem = { id: string; label: string; prompt: string; projectTitle: string }
type ConnectionStart = { nodeId: string; handleType: 'source' | 'target' }
type CanvasImageTool = 'edit' | 'mask' | 'angle'
type CanvasContextAction = CanvasNodeType | 'duplicate' | 'group' | 'ungroup' | 'delete' | `${CanvasImageTool}-image` | `frame-${CanvasVideoFramePosition}`
type CanvasContextMenu =
  | { type: 'pane'; x: number; y: number; flowPosition: { x: number; y: number } }
  | { type: 'connection'; x: number; y: number; flowPosition: { x: number; y: number }; connection: ConnectionStart }
  | { type: 'node'; x: number; y: number; nodeId: string }
  | { type: 'edge'; x: number; y: number; edgeId: string }

const maxNodes = 500
const maxReferenceBytes = 32 * 1024 * 1024
const supportedImageTypes = new Set(['image/png', 'image/jpeg', 'image/webp', 'image/gif'])
const supportedVideoTypes = new Set(['video/mp4', 'video/webm'])
const supportedAudioTypes = new Set(['audio/mpeg', 'audio/mp3', 'audio/wav', 'audio/x-wav', 'audio/ogg', 'audio/opus', 'audio/mp4', 'audio/aac', 'audio/flac'])
const supportedMediaTypes = new Set([...supportedImageTypes, ...supportedVideoTypes, ...supportedAudioTypes])
const store = useCanvasStore()
const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()

const workspaceRef = ref<HTMLElement | null>(null)
const importInput = ref<HTMLInputElement | null>(null)
const referenceInput = ref<HTMLInputElement | null>(null)
const title = ref(t('canvas.untitled'))
const nodes = ref<CanvasFlowNode[]>([])
const edges = ref<CanvasFlowEdge[]>([])
const flowNodes = computed<any[]>({
  get: () => nodes.value,
  set: value => { nodes.value = value as CanvasFlowNode[] },
})
const flowEdges = computed<any[]>({
  get: () => edges.value,
  set: value => { edges.value = value as CanvasFlowEdge[] },
})
const viewport = ref<ViewportTransform>({ x: 0, y: 0, zoom: 1 })
const currentProjectId = ref<number | null>(null)
const selectedNodeId = ref<string | null>(null)
const selectedEdgeId = ref<string | null>(null)
const inspectorTab = ref<InspectorTab>('properties')
const leftPanelOpen = ref(typeof window !== 'undefined' && window.innerWidth > 940)
const rightPanelOpen = ref(false)
const miniMapOpen = ref(true)
const contextMenu = ref<CanvasContextMenu | null>(null)
const viewportSize = ref({ width: 1200, height: 760 })
const mediaUploadTargetId = ref<string | null>(null)
const libraryTab = ref<LibraryTab>('nodes')
const librarySearch = ref('')
const interactionMode = ref<InteractionMode>('pan')
const temporaryInteractionKeys = ref(new Set<string>())
const gridStyle = ref<GridStyle>('dots')
const preferencesOpen = ref(false)
const initializing = ref(true)
const busyProjectAction = ref(false)
const deleteProjectDialogOpen = ref(false)
const dropActive = ref(false)
const saveStatus = ref<SaveStatus>('idle')
const lastSavedAt = ref<Date | null>(null)
const localError = ref('')
const localMessageTone = ref<'error' | 'success'>('error')
const revisions = ref<CanvasRevision[]>([])
const revisionsLoading = ref(false)
const checkpointing = ref(false)
const restoreCandidate = ref<CanvasRevision | null>(null)
const restoring = ref(false)
const runtimeApiKey = ref('')
const showRuntimeApiKey = ref(false)
const uploadingNodeId = ref<string | null>(null)
const generatingNodeId = ref<string | null>(null)
const imageEditorNodeId = ref<string | null>(null)
const imageEditorProcessing = ref(false)
const maskEditorNodeId = ref<string | null>(null)
const maskEditorProcessing = ref(false)
const angleEditorNodeId = ref<string | null>(null)
const angleEditorProcessing = ref(false)
const groupDropTargetId = ref<string | null>(null)
const dirty = ref(false)
const localHistory = ref<LocalHistorySnapshot[]>([])
const localHistoryIndex = ref(-1)
const projectActionLocked = computed(() => busyProjectAction.value
  || store.loading
  || checkpointing.value
  || restoring.value
  || Boolean(generatingNodeId.value)
  || Boolean(uploadingNodeId.value)
  || imageEditorProcessing.value
  || maskEditorProcessing.value
  || angleEditorProcessing.value)
const nodeTypes = {
  prompt: CanvasNodeRenderer,
  reference: CanvasNodeRenderer,
  generation: CanvasNodeRenderer,
  image: CanvasNodeRenderer,
  text: CanvasNodeRenderer,
  video: CanvasNodeRenderer,
  audio: CanvasNodeRenderer,
  config: CanvasNodeRenderer,
  group: CanvasNodeRenderer,
}
const defaultEdgeOptions = {
  type: 'default',
  markerEnd: MarkerType.ArrowClosed,
  interactionWidth: 24,
  selectable: true,
  focusable: true,
}
const connectionLineOptions = { type: ConnectionLineType.Bezier, markerEnd: MarkerType.ArrowClosed }

let flowInstance: VueFlowStore | null = null
let persistTimer: ReturnType<typeof setTimeout> | null = null
let persistPromise: Promise<void> | null = null
let persistQueued = false
let changeVersion = 0
let messageTimer: ReturnType<typeof setTimeout> | null = null
let assetRefreshTimer: ReturnType<typeof setInterval> | null = null
let autoFitTimer: ReturnType<typeof setTimeout> | null = null
let generationController: AbortController | null = null
let imageToolController: AbortController | null = null
let imageToolEpoch = 0
let suppressViewportSave = false
let responsiveFitPending = false
let applyingLocalHistory = false
let pendingConnectionStart: ConnectionStart | null = null
let connectionCompleted = false
let workspaceResizeObserver: ResizeObserver | null = null
let draggedGroupStart: { id: string; x: number; y: number; children: Map<string, { x: number; y: number }> } | null = null
let clipboardFragment: { nodes: CanvasNode[]; edges: CanvasDocument['edges'] } | null = null
const objectUrls = new Set<string>()

const palette = computed<Array<{ type: CanvasNodeType; label: string; description: string; icon: PaletteIcon }>>(() => [
  { type: 'text', label: t('canvas.text'), description: t('canvas.textNodeDescription'), icon: 'type' },
  { type: 'image', label: t('canvas.image'), description: t('canvas.imageNodeDescription'), icon: 'image' },
  { type: 'video', label: t('canvas.videoNode'), description: t('canvas.videoNodeDescription'), icon: 'play' },
  { type: 'audio', label: t('canvas.audio'), description: t('canvas.audioNodeDescription'), icon: 'music' },
  { type: 'config', label: t('canvas.config'), description: t('canvas.configNodeDescription'), icon: 'cog' },
  { type: 'group', label: t('canvas.group'), description: t('canvas.groupNodeDescription'), icon: 'group' },
])
const effectiveInteractionMode = computed<InteractionMode>(() => temporaryInteractionKeys.value.size
  ? interactionMode.value === 'pan' ? 'select' : 'pan'
  : interactionMode.value)
const contextActionPalette = computed(() => {
  const menu = contextMenu.value
  if (menu?.type !== 'connection') return palette.value
  const source = nodes.value.find(node => node.id === menu.connection.nodeId)
  if (!source) return []
  return palette.value.filter(item => item.type !== 'group' && (
    menu.connection.handleType === 'source'
      ? Boolean(getConnectionRule(source.type, item.type))
      : Boolean(getConnectionRule(item.type, source.type))
  ))
})
const libraryTabs = computed<Array<{ id: LibraryTab; label: string }>>(() => [
  { id: 'nodes', label: t('canvas.libraryTabs.nodes') },
  { id: 'assets', label: t('canvas.libraryTabs.assets') },
  { id: 'prompts', label: t('canvas.libraryTabs.prompts') },
])
const gridOptions = computed<Array<{ id: GridStyle; label: string }>>(() => [
  { id: 'dots', label: t('canvas.gridStyles.dots') },
  { id: 'lines', label: t('canvas.gridStyles.lines') },
  { id: 'blank', label: t('canvas.gridStyles.blank') },
])
const normalizedLibrarySearch = computed(() => librarySearch.value.trim().toLocaleLowerCase(locale.value))
const filteredPalette = computed(() => {
  if (!normalizedLibrarySearch.value) return palette.value
  return palette.value.filter(item => `${item.label} ${item.description}`.toLocaleLowerCase(locale.value).includes(normalizedLibrarySearch.value))
})
const selectedNode = computed(() => nodes.value.find(node => node.id === selectedNodeId.value))
const imageEditorNode = computed(() => nodes.value.find(node => node.id === imageEditorNodeId.value))
const maskEditorNode = computed(() => nodes.value.find(node => node.id === maskEditorNodeId.value))
const angleEditorNode = computed(() => nodes.value.find(node => node.id === angleEditorNodeId.value))
const contextMenuNode = computed(() => {
  const menu = contextMenu.value
  return menu?.type === 'node' ? nodes.value.find(node => node.id === menu.nodeId) : undefined
})
const selectedEdge = computed(() => edges.value.find(edge => edge.id === selectedEdgeId.value))
const selectedNodes = computed(() => {
  const selected = nodes.value.filter(node => node.selected)
  if (selected.length) return selected
  const active = selectedNode.value
  return active ? [active] : []
})
const canGroupSelection = computed(() => canGroupCanvasNodes(new Set(selectedNodes.value.map(node => node.id)), nodes.value))
const canUngroupSelection = computed(() => canUngroupCanvasNodes(new Set(selectedNodes.value.map(node => node.id)), nodes.value))
const promptNodes = computed(() => nodes.value.filter(node => node.type === 'prompt'))
const referenceNodes = computed(() => nodes.value.filter(node => node.type === 'reference'))
const generationNodes = computed(() => nodes.value.filter(node => node.type === 'generation'))
const canvasAssets = computed(() => nodes.value.filter(node => ['reference', 'image', 'video', 'audio'].includes(node.type)))
const filteredAssets = computed(() => {
  if (!normalizedLibrarySearch.value) return canvasAssets.value
  return canvasAssets.value.filter(node => String(node.data.label || '').toLocaleLowerCase(locale.value).includes(normalizedLibrarySearch.value))
})
const promptLibrary = computed<PromptLibraryItem[]>(() => {
  const seen = new Set<string>()
  const prompts: PromptLibraryItem[] = []
  for (const project of store.projects) {
    for (const node of project.document.nodes) {
      if (node.type !== 'prompt' && node.type !== 'text') continue
      const prompt = String(node.type === 'text' ? node.data.content : node.data.prompt || '').trim()
      if (!prompt || seen.has(prompt)) continue
      seen.add(prompt)
      prompts.push({
        id: `${project.id}:${node.id}`,
        label: String(node.data.label || t('canvas.promptNodeLabel')),
        prompt,
        projectTitle: project.title,
      })
      if (prompts.length >= 100) return prompts
    }
  }
  return prompts
})
const filteredPrompts = computed(() => {
  if (!normalizedLibrarySearch.value) return promptLibrary.value
  return promptLibrary.value.filter(item => (
    `${item.label} ${item.prompt} ${item.projectTitle}`.toLocaleLowerCase(locale.value).includes(normalizedLibrarySearch.value)
  ))
})
const librarySearchPlaceholder = computed(() => ({
  nodes: t('canvas.searchNodes'),
  assets: t('canvas.searchAssets'),
  prompts: t('canvas.searchPrompts'),
})[libraryTab.value])
const connectedPrompt = computed(() => selectedNode.value?.type === 'generation'
  ? connectedNode(selectedNode.value.id, CANVAS_HANDLES.generationPrompt, nodes.value, edges.value)
  : undefined)
const connectedReference = computed(() => selectedNode.value?.type === 'generation'
  ? connectedNode(selectedNode.value.id, CANVAS_HANDLES.generationReference, nodes.value, edges.value)
  : undefined)
const connectedGeneration = computed(() => selectedNode.value?.type === 'image'
  ? connectedNode(selectedNode.value.id, CANVAS_HANDLES.imageInput, nodes.value, edges.value)
  : undefined)
const selectedConfigHasPrompt = computed(() => selectedNode.value?.type === 'config' && Boolean(resolveGenerationPrompt(selectedNode.value)))
const selectedNodeKind = computed(() => selectedNode.value ? nodeKindLabel(selectedNode.value.type) : '')
const selectedNodeIcon = computed<PaletteIcon>(() => ({
  prompt: 'document',
  reference: 'link',
  generation: 'sparkles',
  image: 'image',
  text: 'type',
  video: 'play',
  audio: 'music',
  config: 'cog',
  group: 'group',
}[selectedNode.value?.type || 'prompt'] as PaletteIcon))
const edgeSourceLabel = computed(() => nodes.value.find(node => node.id === selectedEdge.value?.source)?.data.label || '')
const edgeTargetLabel = computed(() => nodes.value.find(node => node.id === selectedEdge.value?.target)?.data.label || '')
const zoomPercent = computed(() => Math.round(viewport.value.zoom * 100))
const canUndo = computed(() => localHistoryIndex.value > 0)
const canRedo = computed(() => localHistoryIndex.value >= 0 && localHistoryIndex.value < localHistory.value.length - 1)
const visibleMessage = computed(() => localError.value || store.error)
const messageToneResolved = computed(() => store.error ? 'error' : localMessageTone.value)
const saveStateText = computed(() => {
  if (saveStatus.value === 'saving') return t('canvas.saving')
  if (saveStatus.value === 'dirty') return t('canvas.unsavedChanges')
  if (saveStatus.value === 'error') return t('canvas.saveFailed')
  if (saveStatus.value === 'saved' && lastSavedAt.value) {
    return t('canvas.savedAt', { time: lastSavedAt.value.toLocaleTimeString(locale.value, { hour: '2-digit', minute: '2-digit' }) })
  }
  return t('canvas.saved')
})

function nodeKindLabel(type?: string) {
  return ({
    prompt: t('canvas.prompt'),
    reference: t('canvas.reference'),
    generation: t('canvas.generate'),
    image: t('canvas.image'),
    text: t('canvas.text'),
    video: t('canvas.videoNode'),
    audio: t('canvas.audio'),
    config: t('canvas.config'),
    group: t('canvas.group'),
  } as Record<string, string>)[type || ''] || t('canvas.node')
}

function createNodeId(type: CanvasNodeType) {
  return `${type}-${crypto.randomUUID?.() || `${Date.now()}-${Math.random().toString(16).slice(2)}`}`
}

function currentViewport() {
  return flowInstance?.getViewport() || viewport.value
}

function documentFromCanvas(): CanvasDocument {
  return {
    ...serializeCanvasDocument(nodes.value, edges.value, currentViewport()),
    background_mode: gridStyle.value,
  }
}

function localSnapshot(): LocalHistorySnapshot {
  const document = documentFromCanvas()
  const signature = JSON.stringify({ title: title.value, document })
  return { title: title.value, document, signature }
}

function resetLocalHistory() {
  const snapshot = localSnapshot()
  localHistory.value = [snapshot]
  localHistoryIndex.value = 0
}

function recordLocalHistory() {
  if (applyingLocalHistory) return
  const snapshot = localSnapshot()
  if (localHistory.value[localHistoryIndex.value]?.signature === snapshot.signature) return
  const next = localHistory.value.slice(0, localHistoryIndex.value + 1)
  next.push(snapshot)
  if (next.length > 80) next.shift()
  localHistory.value = next
  localHistoryIndex.value = next.length - 1
}

function clearPersistTimer() {
  if (!persistTimer) return
  clearTimeout(persistTimer)
  persistTimer = null
}

async function saveDraftSafely() {
  if (!store.project) return
  try {
    await saveCanvasDraft(store.project.id, title.value, documentFromCanvas())
  } catch {
    // IndexedDB may be unavailable in hardened/private browser contexts.
  }
}

function markCanvasChanged() {
  if (initializing.value || !store.project) return
  recordLocalHistory()
  changeVersion += 1
  dirty.value = true
  saveStatus.value = 'dirty'
  if (persistPromise) persistQueued = true
  void saveDraftSafely()
  clearPersistTimer()
  persistTimer = setTimeout(() => {
    void persistNow().catch(() => undefined)
  }, 900)
}

function applyLocalHistorySnapshot(snapshot: LocalHistorySnapshot) {
  applyingLocalHistory = true
  try {
    title.value = snapshot.title
    applyDocument(snapshot.document)
    changeVersion += 1
    dirty.value = true
    saveStatus.value = 'dirty'
    void saveDraftSafely()
    schedulePersist()
  } finally {
    applyingLocalHistory = false
  }
}

function undoCanvas() {
  if (!canUndo.value) return
  localHistoryIndex.value -= 1
  applyLocalHistorySnapshot(localHistory.value[localHistoryIndex.value])
}

function redoCanvas() {
  if (!canRedo.value) return
  localHistoryIndex.value += 1
  applyLocalHistorySnapshot(localHistory.value[localHistoryIndex.value])
}

async function persistNow() {
  if (!store.project) return
  persistQueued = true
  clearPersistTimer()
  if (persistPromise) return persistPromise

  persistPromise = (async () => {
    while (persistQueued && store.project) {
      persistQueued = false
      const snapshotVersion = changeVersion
      const projectId = store.project.id
      saveStatus.value = 'saving'
      try {
        await store.save(title.value, documentFromCanvas())
        if (store.project?.id !== projectId) return
        await removeCanvasDraft(projectId)
        lastSavedAt.value = new Date()
        dirty.value = changeVersion !== snapshotVersion
        saveStatus.value = dirty.value ? 'dirty' : 'saved'
        if (dirty.value) persistQueued = true
      } catch (error) {
        dirty.value = true
        saveStatus.value = 'error'
        throw error
      }
    }
  })()

  try {
    await persistPromise
  } finally {
    persistPromise = null
  }
}

async function saveFromToolbar() {
  try {
    await persistNow()
  } catch {
    // The persistent error banner provides the recovery action.
  }
}

function setMessage(message: string, tone: 'error' | 'success' = 'error') {
  localError.value = message
  localMessageTone.value = tone
  if (messageTimer) clearTimeout(messageTimer)
  messageTimer = setTimeout(() => {
    localError.value = ''
    messageTimer = null
  }, 5000)
}

function dismissError() {
  localError.value = ''
  store.error = ''
}

function refreshConnectionIndicators() {
  for (const node of nodes.value) {
    if (node.type === 'generation') {
      node.data.promptConnected = Boolean(connectedNode(node.id, CANVAS_HANDLES.generationPrompt, nodes.value, edges.value))
      node.data.referenceConnected = Boolean(connectedNode(node.id, CANVAS_HANDLES.generationReference, nodes.value, edges.value))
    } else if (node.type === 'config') {
      node.data.inputConnected = edges.value.some(edge => edge.target === node.id)
    }
  }
}

const validateConnection: ValidConnectionFunc = (connection, graph) => isCanvasConnectionValid(
  connection,
  graph.nodes,
  graph.edges,
  (connection as CanvasFlowEdge).id,
)

function handleConnect(connection: Connection) {
  const edge = createCanvasEdge(connection, nodes.value, edges.value)
  if (!edge) {
    setMessage(t('canvas.invalidConnection'))
    return
  }
  connectionCompleted = true
  edges.value = [...edges.value, edge]
  selectedNodeId.value = null
  selectedEdgeId.value = edge.id
  refreshConnectionIndicators()
  markCanvasChanged()
}

function handleConnectStart(event: { nodeId?: string; handleType?: 'source' | 'target' }) {
  pendingConnectionStart = event.nodeId && event.handleType
    ? { nodeId: event.nodeId, handleType: event.handleType }
    : null
  connectionCompleted = false
  contextMenu.value = null
}

function handleConnectEnd(event?: MouseEvent) {
  const connection = pendingConnectionStart
  pendingConnectionStart = null
  if (!connection || connectionCompleted || !event) return
  const target = event.target instanceof Element ? event.target : null
  if (target?.closest('.vue-flow__node, .vue-flow__handle')) return
  const flowPosition = flowInstance?.screenToFlowCoordinate({ x: event.clientX, y: event.clientY }) || canvasCenter()
  contextMenu.value = {
    type: 'connection',
    x: event.clientX,
    y: event.clientY,
    flowPosition,
    connection,
  }
}

function connectNodes(source: string, target: string) {
  const edge = createCanvasEdge({ source, target }, nodes.value, edges.value)
  if (!edge) return false
  edges.value = [...edges.value, edge]
  refreshConnectionIndicators()
  return true
}

function replaceIncomingConnection(targetId: string, targetHandle: string, sourceId: string) {
  edges.value = edges.value.filter(edge => !(edge.target === targetId && edge.targetHandle === targetHandle))
  if (sourceId) {
    const edge = createCanvasEdge({ source: sourceId, target: targetId }, nodes.value, edges.value)
    if (!edge) {
      setMessage(t('canvas.invalidConnection'))
      return
    }
    edges.value = [...edges.value, edge]
  }
  refreshConnectionIndicators()
  markCanvasChanged()
}

function setGenerationInput(kind: 'prompt' | 'reference', event: Event) {
  if (!selectedNode.value || selectedNode.value.type !== 'generation') return
  const sourceId = (event.target as HTMLSelectElement).value
  replaceIncomingConnection(
    selectedNode.value.id,
    kind === 'prompt' ? CANVAS_HANDLES.generationPrompt : CANVAS_HANDLES.generationReference,
    sourceId,
  )
}

function setImageInput(event: Event) {
  if (!selectedNode.value || selectedNode.value.type !== 'image') return
  replaceIncomingConnection(
    selectedNode.value.id,
    CANVAS_HANDLES.imageInput,
    (event.target as HTMLSelectElement).value,
  )
}

function handleNodeClick(event: NodeMouseEvent) {
  selectedNodeId.value = event.node.id
  selectedEdgeId.value = null
  inspectorTab.value = 'properties'
  rightPanelOpen.value = true
  if (window.innerWidth <= 940) leftPanelOpen.value = false
}

function handleNodeDragStart(event: NodeDragEvent) {
  setGroupDropTarget(undefined)
  const node = nodes.value.find(item => item.id === event.node.id)
  if (!node || node.type !== 'group') {
    draggedGroupStart = null
    return
  }
  const children = new Map<string, { x: number; y: number }>()
  for (const child of nodes.value) {
    if (child.data.groupId === node.id) children.set(child.id, { ...child.position })
  }
  draggedGroupStart = { id: node.id, x: node.position.x, y: node.position.y, children }
}

function draggedNodeIds(event: NodeDragEvent) {
  const eventNodes = event.nodes?.length ? event.nodes : [event.node]
  return new Set(eventNodes.map(node => node.id))
}

function setGroupDropTarget(groupId?: string) {
  groupDropTargetId.value = groupId || null
  for (const node of nodes.value) {
    if (node.type === 'group') node.data.isGroupDropTarget = node.id === groupId
  }
}

function moveDraggedGroupChildren(event: NodeDragEvent) {
  const drag = draggedGroupStart
  if (!drag || drag.id !== event.node.id) return false
  const dx = event.node.position.x - drag.x
  const dy = event.node.position.y - drag.y
  for (const node of nodes.value) {
    const start = drag.children.get(node.id)
    if (start) node.position = { x: start.x + dx, y: start.y + dy }
  }
  return true
}

function handleNodeDrag(event: NodeDragEvent) {
  if (moveDraggedGroupChildren(event)) {
    setGroupDropTarget(undefined)
    return
  }
  const movedIds = draggedNodeIds(event)
  setGroupDropTarget(findCanvasGroupDropTarget(movedIds, nodes.value)?.id)
}

function handleNodeDragStop(event: NodeDragEvent) {
  const movedGroup = moveDraggedGroupChildren(event)
  const movedIds = draggedNodeIds(event)
  draggedGroupStart = null
  setGroupDropTarget(undefined)
  if (!movedGroup) {
    const target = findCanvasGroupDropTarget(movedIds, nodes.value)
    const updated = target
      ? snapCanvasNodesIntoGroup(movedIds, nodes.value, target)
      : nodes.value.map(node => {
          if (!movedIds.has(node.id) || node.type === 'group') return node
          const groupId = findContainingCanvasGroupId(node, nodes.value)
          if (node.data.groupId === groupId) return node
          return { ...node, data: { ...node.data, groupId } }
        })
    nodes.value = updated.map(attachNodeActions)
  }
  markCanvasChanged()
}

function handleEdgeClick(event: EdgeMouseEvent) {
  selectedEdgeId.value = event.edge.id
  selectedNodeId.value = null
  inspectorTab.value = 'properties'
  rightPanelOpen.value = true
  if (window.innerWidth <= 940) leftPanelOpen.value = false
}

function menuPoint(event: MouseEvent | TouchEvent) {
  if (event instanceof MouseEvent) return { x: event.clientX, y: event.clientY }
  const touch = event.touches[0] || event.changedTouches[0]
  return { x: touch?.clientX || 0, y: touch?.clientY || 0 }
}

function openNodeContextMenu(nodeId: string, event: MouseEvent) {
  event.preventDefault()
  selectOnly([nodeId])
  const point = menuPoint(event)
  contextMenu.value = { type: 'node', nodeId, ...point }
}

function handleNodeContextMenu(event: NodeMouseEvent) {
  openNodeContextMenu(event.node.id, event.event as MouseEvent)
}

function handleEdgeContextMenu(event: EdgeMouseEvent) {
  event.event.preventDefault()
  selectedEdgeId.value = event.edge.id
  selectedNodeId.value = null
  const point = menuPoint(event.event)
  contextMenu.value = { type: 'edge', edgeId: event.edge.id, ...point }
}

function handlePaneContextMenu(event: MouseEvent) {
  event.preventDefault()
  const position = flowInstance?.screenToFlowCoordinate({ x: event.clientX, y: event.clientY }) || canvasCenter()
  contextMenu.value = { type: 'pane', x: event.clientX, y: event.clientY, flowPosition: position }
}

function runContextAction(action: CanvasContextAction) {
  const menu = contextMenu.value
  contextMenu.value = null
  if (!menu) return
  if (menu.type === 'node' && action.endsWith('-image')) {
    const node = nodes.value.find(item => item.id === menu.nodeId)
    if (node) openImageTool(node, action.slice(0, -'-image'.length) as CanvasImageTool)
    return
  }
  if (menu.type === 'node' && action.startsWith('frame-')) {
    const position = action.slice('frame-'.length) as CanvasVideoFramePosition
    const node = nodes.value.find(item => item.id === menu.nodeId)
    if (node) void captureVideoFrameFromNode(node, position)
    return
  }
  if ((menu.type === 'pane' || menu.type === 'connection') && ['prompt', 'reference', 'generation', 'image', 'text', 'video', 'audio', 'config', 'group'].includes(action)) {
    if (menu.type === 'connection') {
      const created = addNode(action as CanvasNodeType, menu.flowPosition, false)
      if (!created) return
      const connected = menu.connection.handleType === 'source'
        ? connectNodes(menu.connection.nodeId, created.id)
        : connectNodes(created.id, menu.connection.nodeId)
      if (!connected) {
        nodes.value = nodes.value.filter(node => node.id !== created.id)
        setMessage(t('canvas.invalidConnection'))
        return
      }
      markCanvasChanged()
      return
    }
    addNode(action as CanvasNodeType, menu.flowPosition, false)
    return
  }
  if (action === 'duplicate' && menu.type === 'node') duplicateSelection([menu.nodeId])
  else if (action === 'group') groupSelection()
  else if (action === 'ungroup') ungroupSelection()
  else if (action === 'delete') deleteSelected()
}

function clearSelection() {
  selectedNodeId.value = null
  selectedEdgeId.value = null
  nodes.value.forEach(node => { node.selected = false })
  edges.value.forEach(edge => { edge.selected = false })
  contextMenu.value = null
}

function deleteSelected() {
  if (selectedEdgeId.value) {
    edges.value = edges.value.filter(edge => edge.id !== selectedEdgeId.value)
    selectedEdgeId.value = null
  } else if (selectedNodes.value.length) {
    const ids = new Set(selectedNodes.value.map(node => node.id))
    nodes.value = nodes.value.filter(node => !ids.has(node.id)).map(node => {
      if (node.data.groupId && ids.has(String(node.data.groupId))) delete node.data.groupId
      return node
    })
    edges.value = edges.value.filter(edge => !ids.has(edge.source) && !ids.has(edge.target))
    if (imageEditorNodeId.value && ids.has(imageEditorNodeId.value)) imageEditorNodeId.value = null
    if (maskEditorNodeId.value && ids.has(maskEditorNodeId.value)) maskEditorNodeId.value = null
    if (angleEditorNodeId.value && ids.has(angleEditorNodeId.value)) angleEditorNodeId.value = null
    selectedNodeId.value = null
  } else {
    return
  }
  refreshConnectionIndicators()
  markCanvasChanged()
}

function selectOnly(nodeIds: Iterable<string>) {
  const ids = new Set(nodeIds)
  nodes.value.forEach(node => { node.selected = ids.has(node.id) })
  edges.value.forEach(edge => { edge.selected = false })
  selectedNodeId.value = ids.size === 1 ? [...ids][0] : null
  selectedEdgeId.value = null
}

function copySelection() {
  const selected = selectedNodes.value
  if (!selected.length) return
  const ids = new Set(selected.map(node => node.id))
  const document = documentFromCanvas()
  clipboardFragment = {
    nodes: document.nodes.filter(node => ids.has(node.id)),
    edges: document.edges.filter(edge => ids.has(edge.source) && ids.has(edge.target)),
  }
  setMessage(t('canvas.copiedNodes', { count: selected.length }), 'success')
}

function pasteSelection() {
  if (!clipboardFragment?.nodes.length) return false
  const available = maxNodes - nodes.value.length
  if (available <= 0) {
    setMessage(t('canvas.nodeLimitReached', { max: maxNodes }))
    return false
  }
  const sourceNodes = clipboardFragment.nodes.slice(0, available)
  const idMap = new Map<string, string>()
  sourceNodes.forEach(node => idMap.set(node.id, createNodeId(node.type as CanvasNodeType)))
  const pasted = sourceNodes.map(source => {
    const clone = structuredClone(source)
    clone.id = idMap.get(source.id)!
    clone.position = { x: source.position.x + 42, y: source.position.y + 42 }
    if (clone.data.groupId) clone.data.groupId = idMap.get(String(clone.data.groupId)) || undefined
    return attachNodeActions({ ...clone, type: clone.type as CanvasNodeType, selected: true } as CanvasFlowNode)
  })
  nodes.value.forEach(node => { node.selected = false })
  nodes.value = [...nodes.value, ...pasted]
  const pastedEdges = clipboardFragment.edges.flatMap(edge => {
    const source = idMap.get(edge.source)
    const target = idMap.get(edge.target)
    if (!source || !target) return []
    const created = createCanvasEdge({ source, target }, nodes.value, edges.value)
    return created ? [created] : []
  })
  edges.value = [...edges.value, ...pastedEdges]
  selectedNodeId.value = pasted.length === 1 ? pasted[0].id : null
  refreshConnectionIndicators()
  markCanvasChanged()
  return true
}

function duplicateSelection(nodeIds?: string[]) {
  if (nodeIds?.length) selectOnly(nodeIds)
  copySelection()
  pasteSelection()
}

function selectAllNodes() {
  selectOnly(nodes.value.map(node => node.id))
}

function groupSelection() {
  const selectedIds = new Set(selectedNodes.value.map(node => node.id))
  const members = collectCanvasGroupMembers(selectedIds, nodes.value)
  if (members.length < 2) return
  const rect = getCanvasGroupWrapRect(members)
  const groupId = createNodeId('group')
  const group = attachNodeActions({
    id: groupId,
    type: 'group',
    position: { x: rect.x, y: rect.y },
    width: Math.max(320, rect.width),
    height: Math.max(220, rect.height),
    data: { label: t('canvas.groupNodeLabel'), collapsed: false },
    dragHandle: '.canvas-node__drag',
    ariaLabel: t('canvas.group'),
    selected: true,
    zIndex: -1,
  })
  const result = applyCanvasGroupSelection(selectedIds, nodes.value, edges.value, group)
  if (!result) return
  nodes.value = result.nodes.map(attachNodeActions)
  edges.value = result.edges
  selectOnly(result.selectedIds)
  markCanvasChanged()
}

function ungroupSelection() {
  const selectedIds = new Set(selectedNodes.value.map(node => node.id))
  const result = applyCanvasUngroupSelection(selectedIds, nodes.value, edges.value)
  if (!result) return
  nodes.value = result.nodes.map(attachNodeActions)
  edges.value = result.edges
  selectOnly(result.selectedIds)
  refreshConnectionIndicators()
  markCanvasChanged()
}

function defaultNodeData(type: CanvasNodeType): CanvasNodeData {
  if (type === 'prompt') return { label: t('canvas.promptNodeLabel'), prompt: '' }
  if (type === 'reference') return { label: t('canvas.referenceNodeLabel') }
  if (type === 'generation') return { label: t('canvas.generationNodeLabel'), model: 'gpt-image-1', generationMode: 'image', status: 'ready' }
  if (type === 'image') return { label: t('canvas.imageNodeLabel') }
  if (type === 'text') return { label: t('canvas.textNodeLabel'), content: '', fontSize: 16 }
  if (type === 'video') return { label: t('canvas.videoNodeLabel') }
  if (type === 'audio') return { label: t('canvas.audioNodeLabel') }
  if (type === 'config') return { label: t('canvas.configNodeLabel'), generationMode: 'image', model: 'gpt-image-1', prompt: '', size: '1024x1024', quality: 'auto', count: 1, status: 'ready' }
  return { label: t('canvas.groupNodeLabel'), collapsed: false }
}

function attachNodeActions(node: CanvasFlowNode) {
  node.data.onChange = () => markCanvasChanged()
  node.data.onOpenProperties = () => {
    selectedNodeId.value = node.id
    selectedEdgeId.value = null
    inspectorTab.value = 'properties'
    rightPanelOpen.value = true
  }
  node.data.onDuplicate = () => duplicateSelection([node.id])
  node.data.onDelete = () => {
    selectOnly([node.id])
    deleteSelected()
  }
  node.data.onResize = (_id: string, width: number, height: number) => {
    node.width = Math.round(width)
    node.height = Math.round(height)
  }
  node.data.onContextMenu = (_id: string, event: MouseEvent) => openNodeContextMenu(node.id, event)
  if (node.type === 'reference' || node.type === 'image' || node.type === 'video' || node.type === 'audio') {
    node.data.onUpload = () => {
      selectedNodeId.value = node.id
      selectedEdgeId.value = null
      mediaUploadTargetId.value = node.id
      void nextTick(() => referenceInput.value?.click())
    }
  }
  if (node.type === 'generation' || node.type === 'config') {
    node.data.onGenerate = () => {
      selectedNodeId.value = node.id
      selectedEdgeId.value = null
      void generateFromNode()
    }
  }
  if (node.type === 'image' || node.type === 'reference' || node.type === 'video' || node.type === 'audio') {
    node.data.onDownload = () => {
      selectedNodeId.value = node.id
      selectedEdgeId.value = null
      void downloadSelectedImage()
    }
  }
  if (node.type === 'image' || node.type === 'reference') {
    node.data.onEdit = () => openImageTool(node, 'edit')
    node.data.onMaskEdit = () => openImageTool(node, 'mask')
    node.data.onAngle = () => openImageTool(node, 'angle')
  }
  return node
}

function openImageTool(node: CanvasFlowNode, tool: CanvasImageTool) {
  if ((node.type !== 'image' && node.type !== 'reference') || !node.data.url) return
  imageEditorNodeId.value = tool === 'edit' ? node.id : null
  maskEditorNodeId.value = tool === 'mask' ? node.id : null
  angleEditorNodeId.value = tool === 'angle' ? node.id : null
}

function canvasCenter() {
  const bounds = workspaceRef.value?.querySelector<HTMLElement>('.canvas-flow')?.getBoundingClientRect()
    || workspaceRef.value?.getBoundingClientRect()
  if (!bounds || !flowInstance) return { x: 160, y: 140 }
  const projected = flowInstance.screenToFlowCoordinate({
    x: bounds.left + bounds.width / 2,
    y: bounds.top + bounds.height / 2,
  })
  return { x: projected.x - 144, y: projected.y - 124 }
}

function preferredNodePosition(type: CanvasNodeType, previousNode?: CanvasFlowNode) {
  if (previousNode && getConnectionRule(previousNode.type, type)) {
    return findAvailableCanvasPosition(
      { x: previousNode.position.x + 380, y: previousNode.position.y },
      nodes.value,
    )
  }

  const downstream = (type === 'prompt' || type === 'reference')
    ? generationNodes.value[0]
    : undefined
  const inputCount = nodes.value.filter(node => node.type === 'prompt' || node.type === 'reference').length
  const preferred = downstream
    ? { x: downstream.position.x - 380, y: downstream.position.y + inputCount * 304 }
    : canvasCenter()
  return findAvailableCanvasPosition(preferred, nodes.value)
}

function scheduleAutoFit() {
  if (autoFitTimer) clearTimeout(autoFitTimer)
  autoFitTimer = setTimeout(() => {
    autoFitTimer = null
    void fitCanvas()
  }, 80)
}

function addNode(type: CanvasNodeType, position?: { x: number; y: number }, autoConnect = true, recordChange = true) {
  if (nodes.value.length >= maxNodes) {
    setMessage(t('canvas.nodeLimitReached', { max: maxNodes }))
    return undefined
  }

  const previousNode = selectedNode.value
  const automaticallyPlaced = !position
  const nextPosition = position || preferredNodePosition(type, previousNode)
  const node = attachNodeActions({
    id: createNodeId(type),
    type,
    position: nextPosition,
    width: CANVAS_NODE_DEFAULT_SIZE[type].width,
    height: CANVAS_NODE_DEFAULT_SIZE[type].height,
    data: defaultNodeData(type),
    dragHandle: '.canvas-node__drag',
    ariaLabel: nodeKindLabel(type),
    zIndex: type === 'group' ? -1 : undefined,
  })
  nodes.value = [...nodes.value, node]
  selectedNodeId.value = node.id
  selectedEdgeId.value = null
  inspectorTab.value = 'properties'
  if (window.innerWidth > 940) rightPanelOpen.value = true

  if (autoConnect && previousNode) {
    if (type === 'generation' && (previousNode.type === 'prompt' || previousNode.type === 'reference')) {
      connectNodes(previousNode.id, node.id)
    } else if (type === 'image' && previousNode.type === 'generation') {
      connectNodes(previousNode.id, node.id)
    }
  }

  refreshConnectionIndicators()
  if (recordChange) markCanvasChanged()
  if (window.innerWidth <= 760) leftPanelOpen.value = false
  if (automaticallyPlaced) scheduleAutoFit()
  return node
}

function createStarterFlow() {
  const center = canvasCenter()
  const text = addNode('text', { x: center.x - 360, y: center.y }, false)
  const config = addNode('config', { x: center.x, y: center.y }, false)
  const image = addNode('image', { x: center.x + 360, y: center.y }, false)
  if (text && config) connectNodes(text.id, config.id)
  if (config && image) connectNodes(config.id, image.id)
  selectedNodeId.value = text?.id || null
  refreshConnectionIndicators()
  markCanvasChanged()
  void nextTick(() => fitCanvas())
}

function handleFlowInit(instance: VueFlowStore) {
  flowInstance = instance
  if (!responsiveFitPending) void nextTick(() => flowInstance?.setViewport(viewport.value))
}

async function handleNodesInitialized() {
  if (!responsiveFitPending || !flowInstance) return
  responsiveFitPending = false
  suppressViewportSave = true
  try {
    await flowInstance.fitView({ padding: 0.18, maxZoom: 0.85, duration: 0 })
    viewport.value = currentViewport()
  } finally {
    suppressViewportSave = false
  }
}

function handleViewportChange(nextViewport: ViewportTransform) {
  viewport.value = nextViewport
  if (!suppressViewportSave) markCanvasChanged()
}

async function zoomCanvasIn() {
  await flowInstance?.zoomIn({ duration: 180 })
  viewport.value = currentViewport()
}

async function zoomCanvasOut() {
  await flowInstance?.zoomOut({ duration: 180 })
  viewport.value = currentViewport()
}

async function fitCanvas() {
  if (!nodes.value.length) return
  await flowInstance?.fitView({ padding: 0.2, maxZoom: 1.2, duration: 220 })
  viewport.value = currentViewport()
  markCanvasChanged()
}

async function resetViewport() {
  await flowInstance?.setViewport({ x: 0, y: 0, zoom: 1 }, { duration: 180 })
  viewport.value = currentViewport()
  markCanvasChanged()
}

async function setZoomFromRange(event: Event) {
  const zoom = Number((event.target as HTMLInputElement).value) / 100
  if (!Number.isFinite(zoom)) return
  const current = currentViewport()
  await flowInstance?.setViewport({ ...current, zoom }, { duration: 120 })
  viewport.value = currentViewport()
  markCanvasChanged()
}

async function setViewportFromMiniMap(nextViewport: ViewportTransform) {
  await flowInstance?.setViewport(nextViewport, { duration: 0 })
  viewport.value = currentViewport()
  markCanvasChanged()
}

function setGridStyle(style: GridStyle) {
  if (gridStyle.value === style) return
  gridStyle.value = style
  localStorage.setItem('modurelay:canvas-grid-style', style)
  markCanvasChanged()
}

async function hydrateAssetUrls() {
  const assetNodes = nodes.value.filter(node =>
    ['reference', 'image', 'video', 'audio'].includes(node.type) && Number(node.data.assetId) > 0,
  )
  await Promise.all(assetNodes.map(async node => {
    try {
      node.data.url = await canvasAPI.assetURL(Number(node.data.assetId))
    } catch {
      delete node.data.url
    }
  }))
}

function applyDocument(document: CanvasDocument) {
  resetImageToolState()
  const normalized = normalizeCanvasDocument(document)
  nodes.value = normalized.nodes.map(attachNodeActions)
  edges.value = normalized.edges
  viewport.value = normalized.viewport
  if (document.background_mode === 'dots' || document.background_mode === 'lines' || document.background_mode === 'blank') {
    gridStyle.value = document.background_mode
  }
  selectedNodeId.value = null
  selectedEdgeId.value = null
  refreshConnectionIndicators()
  responsiveFitPending = window.innerWidth <= 760 && nodes.value.length > 1
  if (!responsiveFitPending) void nextTick(() => flowInstance?.setViewport(normalized.viewport))
  void hydrateAssetUrls()
}

async function loadOpenedProject() {
  if (!store.project) return
  title.value = store.project.title
  currentProjectId.value = store.project.id
  const draft = await getCanvasDraft(store.project.id).catch(() => undefined)
  const serverUpdatedAt = Date.parse(store.project.updated_at)
  const useDraft = Boolean(draft && (!Number.isFinite(serverUpdatedAt) || draft.savedAt > serverUpdatedAt))
  applyDocument(useDraft && draft ? draft.document : store.project.document)
  if (useDraft && draft) {
    title.value = draft.title
    dirty.value = true
    saveStatus.value = 'dirty'
    changeVersion += 1
    schedulePersist()
  } else {
    dirty.value = false
    saveStatus.value = 'saved'
  }
  resetLocalHistory()
  const nextQuery = { ...route.query, project: String(store.project.id) }
  void router.replace({ path: route.path, query: nextQuery }).catch(() => undefined)
}

function schedulePersist() {
  clearPersistTimer()
  persistTimer = setTimeout(() => {
    void persistNow().catch(() => undefined)
  }, 900)
}

async function openProjectById(targetId: number) {
  if (!targetId || targetId === currentProjectId.value) return
  busyProjectAction.value = true
  try {
    if (dirty.value) await persistNow()
    await store.open(targetId)
    await loadOpenedProject()
    revisions.value = []
    libraryTab.value = 'nodes'
    if (window.innerWidth <= 940) leftPanelOpen.value = false
  } catch (error) {
    setMessage(error instanceof Error ? error.message : t('canvas.failedToOpenProject'))
  } finally {
    busyProjectAction.value = false
  }
}

function switchProjectFromSelect(event: Event) {
  void openProjectById(Number((event.target as HTMLSelectElement).value))
}

function addPromptFromLibrary(item: PromptLibraryItem) {
  const node = addNode('text')
  if (!node) return
  node.data.label = item.label
  node.data.content = item.prompt
  markCanvasChanged()
}

function openInNewTab() {
  if (!store.project) return
  const href = router.resolve({ name: 'CanvasEditor', query: { project: String(store.project.id) } }).href
  window.open(href, '_blank', 'noopener,noreferrer')
}

function goToCanvasHome() {
  void router.push({ name: 'CanvasHome' })
}

function focusNode(nodeId: string) {
  const node = nodes.value.find(item => item.id === nodeId)
  if (!node) return
  selectedNodeId.value = node.id
  selectedEdgeId.value = null
  libraryTab.value = 'nodes'
  if (window.innerWidth <= 940) leftPanelOpen.value = false
  void flowInstance?.fitView({ nodes: [node.id], padding: 0.8, maxZoom: 1, duration: 180 })
}

async function createProject() {
  busyProjectAction.value = true
  try {
    if (dirty.value) await persistNow()
    await store.create(t('canvas.untitled'))
    await loadOpenedProject()
    revisions.value = []
  } catch (error) {
    setMessage(error instanceof Error ? error.message : t('canvas.failedToCreateProject'))
  } finally {
    busyProjectAction.value = false
  }
}

async function confirmDeleteProject() {
  if (!store.project) return
  busyProjectAction.value = true
  try {
    const deletedId = store.project.id
    await store.remove(deletedId)
    await removeCanvasDraft(deletedId).catch(() => undefined)
    deleteProjectDialogOpen.value = false
    await router.replace({ name: 'CanvasHome' })
  } catch (error) {
    setMessage(error instanceof Error ? error.message : t('canvas.failedToDeleteProject'))
  } finally {
    busyProjectAction.value = false
  }
}

async function reloadCurrentProject() {
  if (!currentProjectId.value) return
  initializing.value = true
  dismissError()
  try {
    await store.open(currentProjectId.value)
    await removeCanvasDraft(currentProjectId.value).catch(() => undefined)
    await loadOpenedProject()
  } finally {
    initializing.value = false
  }
}

function toggleLeftPanel() {
  leftPanelOpen.value = !leftPanelOpen.value
  if (leftPanelOpen.value && window.innerWidth <= 940) rightPanelOpen.value = false
}

function openProperties() {
  inspectorTab.value = 'properties'
  rightPanelOpen.value = !rightPanelOpen.value
  if (rightPanelOpen.value && window.innerWidth <= 940) leftPanelOpen.value = false
}

async function loadRevisions() {
  if (!store.project) return
  revisionsLoading.value = true
  try {
    revisions.value = await canvasAPI.listRevisions(store.project.id)
  } catch (error) {
    setMessage(error instanceof Error ? error.message : t('canvas.failedToLoadHistory'))
  } finally {
    revisionsLoading.value = false
  }
}

function selectHistoryTab() {
  inspectorTab.value = 'history'
  void loadRevisions()
}

function openHistory() {
  const alreadyOpen = inspectorTab.value === 'history' && rightPanelOpen.value
  inspectorTab.value = 'history'
  rightPanelOpen.value = !alreadyOpen
  if (rightPanelOpen.value) {
    if (window.innerWidth <= 940) leftPanelOpen.value = false
    void loadRevisions()
  }
}

async function createCheckpoint() {
  if (!store.project) return
  checkpointing.value = true
  try {
    if (dirty.value) await persistNow()
    await canvasAPI.checkpoint(store.project.id)
    await loadRevisions()
    setMessage(t('canvas.checkpointCreated'), 'success')
  } catch (error) {
    setMessage(error instanceof Error ? error.message : t('canvas.failedToCreateCheckpoint'))
  } finally {
    checkpointing.value = false
  }
}

async function confirmRestore() {
  if (!store.project || !restoreCandidate.value) return
  restoring.value = true
  try {
    const projectId = store.project.id
    await canvasAPI.restore(projectId, restoreCandidate.value.id, store.project.revision)
    restoreCandidate.value = null
    await removeCanvasDraft(projectId).catch(() => undefined)
    await store.open(projectId)
    await store.list()
    await loadOpenedProject()
    await loadRevisions()
    setMessage(t('canvas.checkpointRestored'), 'success')
  } catch (error) {
    setMessage(error instanceof Error ? error.message : t('canvas.failedToRestoreCheckpoint'))
  } finally {
    restoring.value = false
  }
}

function validateReference(file: File, nodeType: CanvasNodeType) {
  const allowed = nodeType === 'video'
    ? supportedVideoTypes
    : nodeType === 'audio'
      ? supportedAudioTypes
      : supportedImageTypes
  if (!allowed.has(file.type)) return t('canvas.unsupportedMediaType')
  if (file.size <= 0 || file.size > maxReferenceBytes) return t('canvas.referenceTooLarge')
  return ''
}

async function uploadReference(node: CanvasFlowNode, file: File) {
  if (!store.project || !['reference', 'image', 'video', 'audio'].includes(node.type)) return
  const validationError = validateReference(file, node.type)
  if (validationError) {
    setMessage(validationError)
    return
  }

  const previewUrl = URL.createObjectURL(file)
  objectUrls.add(previewUrl)
  node.data.url = previewUrl
  node.data.fileName = file.name
  node.data.contentType = file.type
  node.data.status = 'processing'
  uploadingNodeId.value = node.id
  try {
    const asset = await canvasAPI.uploadAsset(
      store.project.id,
      file,
      `canvas-upload-${store.project.id}-${crypto.randomUUID?.() || Date.now()}`,
    )
    node.data.assetId = asset.id
    node.data.fileName = asset.file_name
    node.data.contentType = asset.content_type
    node.data.url = await canvasAPI.assetURL(asset.id)
    node.data.status = 'succeeded'
    URL.revokeObjectURL(previewUrl)
    objectUrls.delete(previewUrl)
    markCanvasChanged()
  } catch (error) {
    delete node.data.url
    node.data.status = 'failed'
    setMessage(error instanceof Error ? error.message : t('canvas.uploadFailed'))
  } finally {
    uploadingNodeId.value = null
  }
}

async function uploadSelectedReference(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  const node = nodes.value.find(item => item.id === mediaUploadTargetId.value) || selectedNode.value
  input.value = ''
  mediaUploadTargetId.value = null
  if (!file || !node || !['reference', 'image', 'video', 'audio'].includes(node.type)) return
  await uploadReference(node, file)
}

function closeImageEditor() {
  if (!imageEditorProcessing.value) imageEditorNodeId.value = null
}

function closeMaskEditor() {
  if (!maskEditorProcessing.value) maskEditorNodeId.value = null
}

function closeAngleEditor() {
  if (!angleEditorProcessing.value) angleEditorNodeId.value = null
}

function resetImageToolState() {
  imageToolEpoch += 1
  imageToolController?.abort()
  imageToolController = null
  imageEditorNodeId.value = null
  maskEditorNodeId.value = null
  angleEditorNodeId.value = null
  imageEditorProcessing.value = false
  maskEditorProcessing.value = false
  angleEditorProcessing.value = false
}

function imageEditError(error: unknown) {
  const code = error instanceof Error ? error.message : ''
  if (code === 'CANVAS_IMAGE_OUTPUT_TOO_LARGE') return t('canvas.editors.outputTooLarge')
  if (code === 'CANVAS_IMAGE_EDITOR_UNAVAILABLE') return t('canvas.editors.unavailable')
  if (code === 'CANVAS_IMAGE_READ_FAILED') return t('canvas.referenceReadFailed')
  return error instanceof Error ? error.message : t('canvas.editors.failed')
}

function editedFileName(node: CanvasFlowNode, result: CanvasImageEditResult) {
  const original = String(node.data.fileName || 'canvas-image').replace(/\.[^.]+$/, '')
  return `${original}-${result.suffix}.png`
}

async function uploadEditedImage(node: CanvasFlowNode, result: CanvasImageEditResult) {
  const project = store.project
  if (!project) throw new Error(t('canvas.failedToOpenProject'))
  const file = new File([result.blob], editedFileName(node, result), { type: 'image/png' })
  const asset = await canvasAPI.uploadAsset(project.id, file, `canvas-edit-${project.id}-${crypto.randomUUID?.() || Date.now()}`)
  try {
    return { asset, url: await canvasAPI.assetURL(asset.id) }
  } catch (error) {
    await canvasAPI.deleteAsset(asset.id).catch(() => undefined)
    throw error
  }
}

async function removeCanvasAssets(assets: CanvasAsset[]) {
  await Promise.allSettled(assets.map(asset => canvasAPI.deleteAsset(asset.id)))
}

function imageEditSettings(node: CanvasFlowNode) {
  const config = incomingNodes(node.id).find(item => item.type === 'config' || item.type === 'generation')
  return {
    model: String(config?.data.model || node.data.model || 'gpt-image-1'),
    options: {
      size: String(config?.data.size || node.data.size || 'auto'),
      quality: String(config?.data.quality || node.data.quality || 'auto'),
      background: config?.data.background ? String(config.data.background) : undefined,
      count: 1,
    },
  }
}

function imageNodeFromAsset(
  asset: CanvasAsset,
  url: string,
  label: string,
  position: { x: number; y: number },
  data: Partial<CanvasNodeData> = {},
) {
  const node = addNode('image', findAvailableCanvasPosition(position, nodes.value), false, false)
  if (!node) throw new Error(t('canvas.nodeLimitReached', { max: maxNodes }))
  node.data = {
    label,
    assetId: asset.id,
    fileName: asset.file_name,
    contentType: asset.content_type,
    url,
    status: 'succeeded',
    ...data,
  }
  attachNodeActions(node)
  return node
}

function rollbackImageToolNodes(nodeIds: Set<string>, sourceNodeId: string) {
  if (!nodeIds.size) return
  nodes.value = nodes.value.filter(node => !nodeIds.has(node.id))
  edges.value = edges.value.filter(edge => !nodeIds.has(edge.source) && !nodeIds.has(edge.target))
  const source = nodes.value.find(node => node.id === sourceNodeId)
  if (source) selectOnly([source.id])
  else {
    selectedNodeId.value = null
    selectedEdgeId.value = null
  }
  refreshConnectionIndicators()
}

function assertCanvasOperationContext(projectId: number, sourceNodeId: string, operationEpoch = imageToolEpoch) {
  if (operationEpoch !== imageToolEpoch
    || currentProjectId.value !== projectId
    || !nodes.value.some(node => node.id === sourceNodeId)) {
    throw new DOMException('Image tool operation was canceled', 'AbortError')
  }
}

function assertImageToolContext(controller: AbortController, projectId: number, sourceNodeId: string) {
  if (controller.signal.aborted) throw new DOMException('Image tool operation was canceled', 'AbortError')
  assertCanvasOperationContext(projectId, sourceNodeId)
}

function angleNodeLabel(input: CanvasImageAngleParams) {
  const params = normalizeCanvasImageAngle(input)
  const horizontal = params.horizontalAngle === 0
    ? t('canvas.editors.frontView')
    : t(params.horizontalAngle > 0 ? 'canvas.editors.rotateRightAngle' : 'canvas.editors.rotateLeftAngle', { angle: Math.abs(params.horizontalAngle) })
  const pitch = params.pitchAngle === 0
    ? t('canvas.editors.eyeLevel')
    : t(params.pitchAngle > 0 ? 'canvas.editors.topDownAngle' : 'canvas.editors.lowAngle', { angle: Math.abs(params.pitchAngle) })
  return t('canvas.editors.angleResultLabel', {
    horizontal,
    pitch,
    distance: params.cameraDistance.toFixed(1),
    lens: t(params.wideAngle ? 'canvas.editors.wide' : 'canvas.editors.standard'),
  })
}

async function applyImageMask(payload: CanvasImageMaskPayload) {
  const sourceNode = maskEditorNode.value
  const project = store.project
  if (!sourceNode?.data.url || !project || maskEditorProcessing.value) return
  if (payload.generate && !runtimeApiKey.value) {
    setMessage(t('canvas.apiKeyRequired'))
    return
  }
  const addedNodeCount = payload.generate ? 2 : 1
  if (nodes.value.length + addedNodeCount > maxNodes) {
    setMessage(t('canvas.nodeLimitReached', { max: maxNodes }))
    return
  }

  maskEditorProcessing.value = true
  imageToolController?.abort()
  const controller = new AbortController()
  imageToolController = controller
  const signal = controller.signal
  const uploadedAssets: CanvasAsset[] = []
  const createdNodeIds = new Set<string>()
  try {
    let generatedAsset: CanvasAsset | undefined
    let generatedURL = ''
    let generatedTaskId = ''
    const settings = imageEditSettings(sourceNode)
    if (payload.generate) {
      const task = await canvasAPI.submitEdit(
        runtimeApiKey.value,
        payload.prompt,
        settings.model,
        await generationReferenceBlob(sourceNode, signal),
        String(sourceNode.data.fileName || 'source.png'),
        { ...settings.options, mask: payload.mask, maskFileName: 'mask.png' },
        signal,
      )
      const completed = await waitForImageResult(runtimeApiKey.value, task, signal)
      assertImageToolContext(controller, project.id, sourceNode.id)
      generatedTaskId = completed.taskId
      generatedAsset = await canvasAPI.promoteTaskAsset(
        project.id,
        completed.taskId,
        0,
        `canvas-mask-result-${project.id}-${completed.taskId}`,
      )
      uploadedAssets.push(generatedAsset)
      generatedURL = await canvasAPI.assetURL(generatedAsset.id)
      assertImageToolContext(controller, project.id, sourceNode.id)
    }

    const maskFile = new File([payload.mask], 'mask.png', { type: 'image/png' })
    const maskAsset = await canvasAPI.uploadAsset(project.id, maskFile, `canvas-mask-${project.id}-${crypto.randomUUID?.() || Date.now()}`)
    uploadedAssets.push(maskAsset)
    const maskURL = await canvasAPI.assetURL(maskAsset.id)
    assertImageToolContext(controller, project.id, sourceNode.id)
    const maskNode = imageNodeFromAsset(
      maskAsset,
      maskURL,
      t('canvas.editors.maskNodeLabel'),
      { x: sourceNode.position.x, y: sourceNode.position.y + (Number(sourceNode.height) || 280) + 80 },
      { naturalWidth: payload.width, naturalHeight: payload.height },
    )
    createdNodeIds.add(maskNode.id)

    let selected = maskNode
    if (generatedAsset) {
      const resultNode = imageNodeFromAsset(
        generatedAsset,
        generatedURL,
        payload.prompt.slice(0, 32) || t('canvas.editors.maskResultLabel'),
        { x: sourceNode.position.x + (Number(sourceNode.width) || 320) + 80, y: sourceNode.position.y },
        {
          taskId: generatedTaskId,
          prompt: payload.prompt,
          model: settings.model,
          size: settings.options.size,
          quality: settings.options.quality,
        },
      )
      createdNodeIds.add(resultNode.id)
      connectNodes(sourceNode.id, resultNode.id)
      connectNodes(maskNode.id, resultNode.id)
      selected = resultNode
    } else {
      connectNodes(sourceNode.id, maskNode.id)
    }
    selectOnly([selected.id])
    maskEditorNodeId.value = null
    refreshConnectionIndicators()
    markCanvasChanged()
    setMessage(payload.generate ? t('canvas.editors.maskGenerated') : t('canvas.editors.maskSaved'), 'success')
  } catch (error) {
    rollbackImageToolNodes(createdNodeIds, sourceNode.id)
    if (uploadedAssets.length) await removeCanvasAssets(uploadedAssets)
    if (!(error instanceof DOMException && error.name === 'AbortError')) setMessage(imageEditError(error))
  } finally {
    if (imageToolController === controller) {
      maskEditorProcessing.value = false
      imageToolController = null
    }
  }
}

async function applyImageAngle(input: CanvasImageAngleParams) {
  const sourceNode = angleEditorNode.value
  const project = store.project
  if (!sourceNode?.data.url || !project || angleEditorProcessing.value) return
  if (!runtimeApiKey.value) {
    setMessage(t('canvas.apiKeyRequired'))
    return
  }
  if (nodes.value.length >= maxNodes) {
    setMessage(t('canvas.nodeLimitReached', { max: maxNodes }))
    return
  }

  angleEditorProcessing.value = true
  imageToolController?.abort()
  const controller = new AbortController()
  imageToolController = controller
  const signal = controller.signal
  let promotedAsset: CanvasAsset | undefined
  const createdNodeIds = new Set<string>()
  try {
    const params = normalizeCanvasImageAngle(input)
    const prompt = buildCanvasImageAnglePrompt(params)
    const settings = imageEditSettings(sourceNode)
    const task = await canvasAPI.submitEdit(
      runtimeApiKey.value,
      prompt,
      settings.model,
      await generationReferenceBlob(sourceNode, signal),
      String(sourceNode.data.fileName || 'source.png'),
      settings.options,
      signal,
    )
    const completed = await waitForImageResult(runtimeApiKey.value, task, signal)
    promotedAsset = await canvasAPI.promoteTaskAsset(
      project.id,
      completed.taskId,
      0,
      `canvas-angle-result-${project.id}-${completed.taskId}`,
    )
    const resultURL = await canvasAPI.assetURL(promotedAsset.id)
    assertImageToolContext(controller, project.id, sourceNode.id)
    const resultNode = imageNodeFromAsset(
      promotedAsset,
      resultURL,
      angleNodeLabel(params),
      { x: sourceNode.position.x + (Number(sourceNode.width) || 320) + 80, y: sourceNode.position.y },
      {
        taskId: completed.taskId,
        prompt,
        model: settings.model,
        size: settings.options.size,
        quality: settings.options.quality,
      },
    )
    createdNodeIds.add(resultNode.id)
    connectNodes(sourceNode.id, resultNode.id)
    selectOnly([resultNode.id])
    angleEditorNodeId.value = null
    refreshConnectionIndicators()
    markCanvasChanged()
    setMessage(t('canvas.editors.angleGenerated'), 'success')
  } catch (error) {
    rollbackImageToolNodes(createdNodeIds, sourceNode.id)
    if (promotedAsset) await removeCanvasAssets([promotedAsset])
    if (!(error instanceof DOMException && error.name === 'AbortError')) setMessage(imageEditError(error))
  } finally {
    if (imageToolController === controller) {
      angleEditorProcessing.value = false
      imageToolController = null
    }
  }
}

async function captureVideoFrameFromNode(node: CanvasFlowNode, position: CanvasVideoFramePosition) {
  if (node.type !== 'video' || !node.data.url) return
  if (nodes.value.length >= maxNodes) {
    setMessage(t('canvas.nodeLimitReached', { max: maxNodes }))
    return
  }
  uploadingNodeId.value = node.id
  try {
    const renderedNode = Array.from(workspaceRef.value?.querySelectorAll<HTMLElement>('.vue-flow__node') || [])
      .find(element => element.dataset.id === node.id)
    const currentTime = renderedNode?.querySelector<HTMLVideoElement>('video')?.currentTime || 0
    const frame = await captureCanvasVideoFrame(String(node.data.url), position, currentTime)
    const result: CanvasImageEditResult = { ...frame, suffix: `frame-${position}` }
    const { asset, url } = await uploadEditedImage(node, result)
    const child = addNode('image', findAvailableCanvasPosition({ x: node.position.x + 380, y: node.position.y }, nodes.value), false, false)
    if (!child) throw new Error(t('canvas.nodeLimitReached', { max: maxNodes }))
    child.data = {
      label: t(`canvas.videoFrames.${position}`),
      assetId: asset.id,
      fileName: asset.file_name,
      contentType: asset.content_type,
      url,
      naturalWidth: frame.width,
      naturalHeight: frame.height,
      status: 'succeeded',
    }
    attachNodeActions(child)
    connectNodes(node.id, child.id)
    refreshConnectionIndicators()
    markCanvasChanged()
    setMessage(t('canvas.videoFrames.saved'), 'success')
  } catch (error) {
    setMessage(imageEditError(error))
  } finally {
    uploadingNodeId.value = null
  }
}

async function applyImageEdit(operation: CanvasImageEditOperation) {
  const node = imageEditorNode.value
  const project = store.project
  if (!node?.data.url || !project || imageEditorProcessing.value) return
  imageEditorProcessing.value = true
  const operationEpoch = ++imageToolEpoch
  const uploadedAssets: CanvasAsset[] = []
  const createdNodeIds = new Set<string>()
  try {
    const results = await editCanvasImage(String(node.data.url), operation)
    assertCanvasOperationContext(project.id, node.id, operationEpoch)
    if (operation.mode === 'split' && nodes.value.length + results.length > maxNodes) {
      throw new Error(t('canvas.nodeLimitReached', { max: maxNodes }))
    }

    if (operation.mode === 'split') {
      const columns = Math.max(1, Math.min(6, Math.round(operation.columns)))
      const uploaded: Array<{ result: CanvasImageEditResult; asset: CanvasAsset; url: string }> = []
      for (const result of results) {
        const upload = await uploadEditedImage(node, result)
        uploaded.push({ result, ...upload })
        uploadedAssets.push(upload.asset)
        assertCanvasOperationContext(project.id, node.id, operationEpoch)
      }
      for (let index = 0; index < uploaded.length; index += 1) {
        const { result, asset, url } = uploaded[index]
        const preferred = {
          x: node.position.x + 380 + (index % columns) * 360,
          y: node.position.y + Math.floor(index / columns) * 300,
        }
        const child = addNode('image', findAvailableCanvasPosition(preferred, nodes.value), false, false)
        if (!child) throw new Error(t('canvas.nodeLimitReached', { max: maxNodes }))
        createdNodeIds.add(child.id)
        child.data = {
          label: `${String(node.data.label || t('canvas.image'))} ${result.suffix}`,
          assetId: asset.id,
          fileName: asset.file_name,
          contentType: asset.content_type,
          url,
          naturalWidth: result.width,
          naturalHeight: result.height,
          status: 'succeeded',
        }
        attachNodeActions(child)
        connectNodes(node.id, child.id)
      }
    } else {
      const result = results[0]
      if (!result) throw new Error(t('canvas.editors.failed'))
      const { asset, url } = await uploadEditedImage(node, result)
      uploadedAssets.push(asset)
      assertCanvasOperationContext(project.id, node.id, operationEpoch)
      node.data.assetId = asset.id
      node.data.fileName = asset.file_name
      node.data.contentType = asset.content_type
      node.data.url = url
      node.data.naturalWidth = result.width
      node.data.naturalHeight = result.height
      node.data.status = 'succeeded'
    }
    imageEditorNodeId.value = null
    refreshConnectionIndicators()
    markCanvasChanged()
    uploadedAssets.length = 0
    setMessage(t('canvas.editors.saved'), 'success')
  } catch (error) {
    rollbackImageToolNodes(createdNodeIds, node.id)
    if (uploadedAssets.length) await removeCanvasAssets(uploadedAssets)
    if (!(error instanceof DOMException && error.name === 'AbortError')) setMessage(imageEditError(error))
  } finally {
    if (imageToolEpoch === operationEpoch) imageEditorProcessing.value = false
  }
}

function handleDragLeave(event: DragEvent) {
  const nextTarget = event.relatedTarget as Node | null
  if (!workspaceRef.value?.contains(nextTarget)) dropActive.value = false
}

async function handleCanvasDrop(event: DragEvent) {
  dropActive.value = false
  const file = Array.from(event.dataTransfer?.files || []).find(candidate => supportedMediaTypes.has(candidate.type))
  if (!file) {
    setMessage(t('canvas.unsupportedReferenceType'))
    return
  }
  const position = flowInstance?.screenToFlowCoordinate({ x: event.clientX, y: event.clientY }) || canvasCenter()
  const type: CanvasNodeType = supportedVideoTypes.has(file.type) ? 'video' : supportedAudioTypes.has(file.type) ? 'audio' : 'image'
  const node = addNode(type, position, false)
  if (node) await uploadReference(node, file)
}

async function pasteSystemClipboard() {
  try {
    const payload = await readCanvasClipboard(navigator.clipboard)
    if (!payload) return
    if (payload.kind === 'image') {
      const node = addNode('image', canvasCenter(), false)
      if (!node) return
      await uploadReference(node, payload.file)
      if (node.data.assetId) setMessage(t('canvas.clipboardImageAdded'), 'success')
      return
    }
    const node = addNode('text', canvasCenter(), false)
    if (!node) return
    node.data.label = payload.text.slice(0, 32) || t('canvas.textNodeLabel')
    node.data.content = payload.text.slice(0, 100_000)
    markCanvasChanged()
    setMessage(t('canvas.clipboardTextAdded'), 'success')
  } catch {
    setMessage(t('canvas.clipboardReadFailed'))
  }
}

function handleConfigModeChange() {
  const node = selectedNode.value
  if (!node || node.type !== 'config') return
  const mode = node.data.generationMode || 'image'
  if (mode === 'video') {
    node.data.model = 'grok-imagine-video-1.5'
    node.data.aspectRatio ||= '16:9'
    node.data.resolution ||= '720p'
    node.data.seconds ||= 6
  } else if (mode === 'audio') {
    node.data.model = CANVAS_GROK_TTS_MODEL
    node.data.voice = CANVAS_GROK_TTS_VOICES.includes(node.data.voice as typeof CANVAS_GROK_TTS_VOICES[number])
      ? node.data.voice
      : CANVAS_GROK_TTS_VOICES[0]
    node.data.language ||= 'en'
  } else if (mode === 'text') {
    node.data.model = 'gpt-4.1-mini'
  } else {
    node.data.model = 'gpt-image-1'
    node.data.size ||= '1024x1024'
    node.data.quality ||= 'auto'
    node.data.count ||= 1
  }
  markCanvasChanged()
}

function closeContextMenu() {
  contextMenu.value = null
}

function taskErrorMessage(error: unknown) {
  if (typeof error === 'string') return error.slice(0, 400)
  if (error && typeof error === 'object' && 'message' in error) return String((error as { message: unknown }).message).slice(0, 400)
  return t('canvas.generationFailed')
}

function waitForPoll(ms: number, signal: AbortSignal) {
  return new Promise<void>((resolve, reject) => {
    const timer = setTimeout(resolve, ms)
    signal.addEventListener('abort', () => {
      clearTimeout(timer)
      reject(new DOMException('Aborted', 'AbortError'))
    }, { once: true })
  })
}

async function generationReferenceBlob(reference: CanvasFlowNode, signal: AbortSignal) {
  if (!reference.data.assetId) throw new Error(t('canvas.referenceUploadRequired'))
  const url = await canvasAPI.assetURL(Number(reference.data.assetId))
  const response = await fetch(url, { signal })
  if (!response.ok) throw new Error(t('canvas.referenceReadFailed'))
  return response.blob()
}

function incomingNodes(nodeId: string) {
  const nodeById = new Map(nodes.value.map(node => [node.id, node]))
  return edges.value
    .filter(edge => edge.target === nodeId)
    .map(edge => nodeById.get(edge.source))
    .filter((node): node is CanvasFlowNode => Boolean(node))
}

function resolveGenerationPrompt(node: CanvasFlowNode) {
  const ownPrompt = String(node.data.prompt || node.data.composerContent || '').trim()
  if (ownPrompt) return ownPrompt
  const input = incomingNodes(node.id).find(item => item.type === 'prompt' || item.type === 'text')
  return String(input?.data.prompt || input?.data.content || '').trim()
}

function resolveGenerationReference(node: CanvasFlowNode) {
  if (node.type === 'generation') return connectedReference.value
  return incomingNodes(node.id).find(item => (item.type === 'reference' || item.type === 'image') && item.data.assetId)
}

function videoTaskId(task: VideoGenerationTask) {
  return String(task.request_id || task.id || '').trim()
}

function videoTaskStatus(task: VideoGenerationTask) {
  return String(task.status || '').trim().toLowerCase()
}

async function waitForVideoResult(apiKey: string, taskId: string, signal: AbortSignal) {
  for (let attempt = 0; attempt < 120; attempt += 1) {
    const task = await videoAPI.status(apiKey, taskId, signal)
    const status = videoTaskStatus(task)
    if (['done', 'completed', 'succeeded', 'success'].includes(status)) return videoAPI.content(apiKey, taskId, signal)
    if (['failed', 'error', 'expired', 'canceled', 'cancelled'].includes(status)) {
      throw new Error(taskErrorMessage(task.error))
    }
    await waitForPoll(3000, signal)
  }
  throw new Error(t('canvas.generationTimedOut'))
}

async function waitForImageResult(apiKey: string, initial: ImageTask, signal: AbortSignal) {
  const taskId = String(initial.task_id || initial.id || '').trim()
  if (!taskId) throw new Error(t('canvas.invalidTask'))
  let latest = initial
  for (let attempt = 0; attempt <= 60; attempt += 1) {
    const status = String(latest.status || '').toLowerCase()
    if (['completed', 'succeeded', 'success'].includes(status)) return { taskId, task: latest }
    if (['failed', 'error', 'expired', 'canceled', 'cancelled'].includes(status)) {
      throw new Error(taskErrorMessage(latest.error))
    }
    if (attempt === 60) break
    await waitForPoll(3000, signal)
    latest = await canvasAPI.getImageTask(apiKey, taskId, signal)
  }
  throw new Error(t('canvas.generationTimedOut'))
}

async function createUploadedResultNode(
  origin: CanvasFlowNode,
  type: 'video' | 'audio',
  blob: Blob,
  fileName: string,
  index = 0,
) {
  const project = store.project
  if (!project) throw new Error(t('canvas.failedToOpenProject'))
  const file = new File([blob], fileName, { type: blob.type || 'application/octet-stream' })
  const asset = await canvasAPI.uploadAsset(project.id, file, `canvas-generated-${project.id}-${crypto.randomUUID?.() || Date.now()}`)
  const resultNode = addNode(type, { x: origin.position.x + 380, y: origin.position.y + index * 220 }, false)
  if (!resultNode) throw new Error(t('canvas.nodeLimitReached', { max: maxNodes }))
  resultNode.data = {
    label: type === 'video' ? t('canvas.generatedVideo') : t('canvas.generatedAudio'),
    assetId: asset.id,
    fileName: asset.file_name,
    contentType: asset.content_type,
    url: await canvasAPI.assetURL(asset.id),
    status: 'succeeded',
  }
  attachNodeActions(resultNode)
  connectNodes(origin.id, resultNode.id)
  return resultNode
}

async function generateFromNode() {
  const node = selectedNode.value
  const project = store.project
  if (!node || (node.type !== 'generation' && node.type !== 'config') || !project) return
  const prompt = node.type === 'generation'
    ? String(connectedPrompt.value?.data.prompt || '').trim()
    : resolveGenerationPrompt(node)
  if (!prompt) {
    setMessage(t('canvas.promptRequired'))
    return
  }
  if (!runtimeApiKey.value) {
    setMessage(t('canvas.apiKeyRequired'))
    return
  }

  generationController?.abort()
  generationController = new AbortController()
  const signal = generationController.signal
  generatingNodeId.value = node.id
  delete node.data.error
  node.data.status = 'queued'
  markCanvasChanged()

  try {
    const model = String(node.data.model || 'gpt-image-1')
    const mode = node.type === 'generation' ? 'image' : node.data.generationMode || 'image'
    let lastResultNode: CanvasFlowNode | undefined

    if (mode === 'video') {
      const task = await videoAPI.submit(runtimeApiKey.value, {
        model,
        prompt,
        duration: Number(node.data.seconds) || 6,
        aspect_ratio: String(node.data.aspectRatio || '16:9'),
        resolution: String(node.data.resolution || '720p'),
      }, signal)
      const taskId = videoTaskId(task)
      if (!taskId) throw new Error(t('canvas.invalidTask'))
      node.data.taskId = taskId
      node.data.status = 'processing'
      markCanvasChanged()
      const blob = await waitForVideoResult(runtimeApiKey.value, taskId, signal)
      lastResultNode = await createUploadedResultNode(node, 'video', blob, `generated-${taskId}.mp4`)
    } else if (mode === 'audio') {
      const blob = await canvasAPI.submitSpeech(runtimeApiKey.value, {
        prompt,
        voice: String(node.data.voice || CANVAS_GROK_TTS_VOICES[0]),
        language: String(node.data.language || 'en'),
      }, signal)
      lastResultNode = await createUploadedResultNode(node, 'audio', blob, 'generated-audio.mp3')
    } else if (mode === 'text') {
      const content = await canvasAPI.submitText(runtimeApiKey.value, prompt, model, signal)
      const textNode = addNode('text', { x: node.position.x + 380, y: node.position.y }, false)
      if (!textNode) throw new Error(t('canvas.nodeLimitReached', { max: maxNodes }))
      textNode.data = { label: t('canvas.generatedText'), content, fontSize: 16, status: 'succeeded' }
      attachNodeActions(textNode)
      connectNodes(node.id, textNode.id)
      lastResultNode = textNode
    } else {
      const options = {
        size: String(node.data.size || '1024x1024'),
        quality: String(node.data.quality || 'auto'),
        background: node.data.background ? String(node.data.background) : undefined,
        count: Number(node.data.count) || 1,
      }
      const reference = resolveGenerationReference(node)
      const task = reference
        ? await canvasAPI.submitEdit(
            runtimeApiKey.value,
            prompt,
            model,
            await generationReferenceBlob(reference, signal),
            String(reference.data.fileName || 'reference.png'),
            options,
            signal,
          )
        : await canvasAPI.submitGeneration(runtimeApiKey.value, prompt, model, options, signal)
      const taskId = String(task.task_id || task.id || '').trim()
      if (!taskId) throw new Error(t('canvas.invalidTask'))
      node.data.taskId = taskId
      node.data.status = task.status || 'queued'
      markCanvasChanged()

      const completed = await waitForImageResult(runtimeApiKey.value, task, signal)
      node.data.status = completed.task.status
      const resultCount = Math.max(1, Math.min(4, completed.task.result?.data?.length || options.count))
      for (let index = 0; index < resultCount; index += 1) {
        const asset = await canvasAPI.promoteTaskAsset(project.id, taskId, index, `canvas-task-${project.id}-${taskId}-${index}`)
        const imageNode = addNode('image', { x: node.position.x + 380, y: node.position.y + index * 300 }, false, false)
        if (!imageNode) throw new Error(t('canvas.nodeLimitReached', { max: maxNodes }))
        imageNode.data = {
          label: resultCount > 1 ? `${t('canvas.generatedResult')} ${index + 1}` : t('canvas.generatedResult'),
          assetId: asset.id,
          fileName: asset.file_name,
          contentType: asset.content_type,
          taskId,
          url: await canvasAPI.assetURL(asset.id),
          status: 'succeeded',
          prompt,
          model,
          size: options.size,
          quality: options.quality,
        }
        attachNodeActions(imageNode)
        connectNodes(node.id, imageNode.id)
        lastResultNode = imageNode
      }
    }

    node.data.status = 'succeeded'
    if (lastResultNode) selectedNodeId.value = lastResultNode.id
    refreshConnectionIndicators()
    markCanvasChanged()
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') return
    node.data.status = 'failed'
    node.data.error = error instanceof Error ? error.message : t('canvas.generationFailed')
    setMessage(String(node.data.error))
    markCanvasChanged()
  } finally {
    generatingNodeId.value = null
    generationController = null
  }
}

async function downloadSelectedImage() {
  const node = selectedNode.value
  if (!node?.data.url) return
  try {
    const response = await fetch(String(node.data.url))
    if (!response.ok) throw new Error(t('canvas.downloadFailed'))
    const url = URL.createObjectURL(await response.blob())
    const link = document.createElement('a')
    link.href = url
    link.download = String(node.data.fileName || 'canvas-image.png')
    link.click()
    URL.revokeObjectURL(url)
  } catch (error) {
    setMessage(error instanceof Error ? error.message : t('canvas.downloadFailed'))
  }
}

function exportProject() {
  if (!store.project) return
  const payload = JSON.stringify({
    format: 'modurelay-canvas',
    version: 1,
    title: title.value,
    document: documentFromCanvas(),
  }, null, 2)
  const blob = new Blob([payload], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `${title.value || t('canvas.untitled')}.canvas`
  link.click()
  URL.revokeObjectURL(url)
}

async function importProject(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  try {
    if (file.size > 5 * 1024 * 1024) throw new Error(t('canvas.importTooLarge'))
    const parsed = JSON.parse(await file.text()) as unknown
    const record = parsed && typeof parsed === 'object' ? parsed as Record<string, unknown> : null
    const candidate = record?.document && typeof record.document === 'object'
      ? record.document as Record<string, unknown>
      : record
    if (!candidate || !Array.isArray(candidate.nodes) || !Array.isArray(candidate.edges)) throw new Error(t('canvas.invalidFile'))
    if (typeof record?.title === 'string') title.value = record.title.slice(0, 160)
    applyDocument(candidate as unknown as CanvasDocument)
    changeVersion += 1
    dirty.value = true
    saveStatus.value = 'dirty'
    await saveDraftSafely()
    schedulePersist()
  } catch (error) {
    setMessage(error instanceof Error ? error.message : t('canvas.invalidFile'))
  }
}

function formatTimestamp(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(locale.value, {
    year: 'numeric',
    month: 'short',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

function isTextInput(target: EventTarget | null) {
  const element = target as HTMLElement | null
  if (!element) return false
  return ['INPUT', 'TEXTAREA', 'SELECT'].includes(element.tagName) || element.isContentEditable
}

function temporaryInteractionKey(event: KeyboardEvent) {
  if (event.key === ' ' || event.code === 'Space') return 'Space'
  if (event.key === 'Control') return 'Control'
  if (event.key === 'Meta') return 'Meta'
  return ''
}

function setTemporaryInteractionKey(key: string, active: boolean) {
  if (!key) return
  const next = new Set(temporaryInteractionKeys.value)
  if (active) next.add(key)
  else next.delete(key)
  temporaryInteractionKeys.value = next
}

function handleKeyboard(event: KeyboardEvent) {
  const temporaryKey = temporaryInteractionKey(event)
  if (!isTextInput(event.target) && temporaryKey) {
    if (temporaryKey === 'Space') event.preventDefault()
    setTemporaryInteractionKey(temporaryKey, true)
    return
  }
  const modifier = event.ctrlKey || event.metaKey
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
    event.preventDefault()
    void saveFromToolbar()
    return
  }
  if (event.key === 'Escape') {
    clearSelection()
    if (window.innerWidth <= 940) leftPanelOpen.value = false
    rightPanelOpen.value = false
    preferencesOpen.value = false
    contextMenu.value = null
    return
  }
  if (!isTextInput(event.target) && modifier && event.key.toLowerCase() === 'a') {
    event.preventDefault()
    selectAllNodes()
    return
  }
  if (!isTextInput(event.target) && modifier && event.key.toLowerCase() === 'c') {
    event.preventDefault()
    copySelection()
    return
  }
  if (!isTextInput(event.target) && modifier && event.key.toLowerCase() === 'v') {
    event.preventDefault()
    if (!pasteSelection()) void pasteSystemClipboard()
    return
  }
  if (!isTextInput(event.target) && modifier && event.key.toLowerCase() === 'd') {
    event.preventDefault()
    duplicateSelection()
    return
  }
  if (!isTextInput(event.target) && modifier && event.key.toLowerCase() === 'g') {
    event.preventDefault()
    if (event.shiftKey) ungroupSelection()
    else groupSelection()
    return
  }
  if (!isTextInput(event.target) && (event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'z') {
    event.preventDefault()
    if (event.shiftKey) redoCanvas()
    else undoCanvas()
    return
  }
  if (!isTextInput(event.target) && (event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'y') {
    event.preventDefault()
    redoCanvas()
    return
  }
  if (!isTextInput(event.target) && event.key.toLowerCase() === 'h') {
    interactionMode.value = 'pan'
    return
  }
  if (!isTextInput(event.target) && event.key.toLowerCase() === 'v') {
    interactionMode.value = 'select'
    return
  }
  if (!isTextInput(event.target) && (event.key === 'Delete' || event.key === 'Backspace')) {
    event.preventDefault()
    deleteSelected()
  }
}

function handleKeyboardUp(event: KeyboardEvent) {
  setTemporaryInteractionKey(temporaryInteractionKey(event), false)
}

function clearTemporaryInteractionKeys() {
  temporaryInteractionKeys.value = new Set()
}

function handleBeforeUnload(event: BeforeUnloadEvent) {
  if (!dirty.value) return
  event.preventDefault()
  event.returnValue = ''
}

onBeforeRouteLeave(async () => {
  if (dirty.value) {
    try {
      await persistNow()
    } catch {
      return false
    }
  }
  return true
})

onMounted(async () => {
  const savedGridStyle = localStorage.getItem('modurelay:canvas-grid-style')
  if (savedGridStyle === 'dots' || savedGridStyle === 'lines' || savedGridStyle === 'blank') {
    gridStyle.value = savedGridStyle
  }
  window.addEventListener('keydown', handleKeyboard)
  window.addEventListener('keyup', handleKeyboardUp)
  window.addEventListener('blur', clearTemporaryInteractionKeys)
  window.addEventListener('beforeunload', handleBeforeUnload)
  window.addEventListener('pointerdown', closeContextMenu)
  if (workspaceRef.value && typeof ResizeObserver !== 'undefined') {
    workspaceResizeObserver = new ResizeObserver(entries => {
      const bounds = entries[0]?.contentRect
      if (bounds) viewportSize.value = { width: bounds.width, height: bounds.height }
    })
    workspaceResizeObserver.observe(workspaceRef.value)
  }
  try {
    await store.list()
    const requestedProjectId = Number(route.query.project)
    const requestedProject = Number.isSafeInteger(requestedProjectId)
      ? store.projects.find(project => project.id === requestedProjectId)
      : undefined
    const first = requestedProject || store.projects[0] || await store.create(t('canvas.untitled'))
    await store.open(first.id)
    await loadOpenedProject()
    assetRefreshTimer = setInterval(() => void hydrateAssetUrls(), 10 * 60 * 1000)
  } catch (error) {
    setMessage(error instanceof Error ? error.message : t('canvas.failedToLoadProjects'))
  } finally {
    initializing.value = false
  }
})

onBeforeUnmount(() => {
  generationController?.abort()
  imageToolController?.abort()
  clearPersistTimer()
  if (autoFitTimer) clearTimeout(autoFitTimer)
  if (messageTimer) clearTimeout(messageTimer)
  if (assetRefreshTimer) clearInterval(assetRefreshTimer)
  workspaceResizeObserver?.disconnect()
  workspaceResizeObserver = null
  for (const url of objectUrls) URL.revokeObjectURL(url)
  objectUrls.clear()
  window.removeEventListener('keydown', handleKeyboard)
  window.removeEventListener('keyup', handleKeyboardUp)
  window.removeEventListener('blur', clearTemporaryInteractionKeys)
  window.removeEventListener('beforeunload', handleBeforeUnload)
  window.removeEventListener('pointerdown', closeContextMenu)
  if (dirty.value && store.project) void saveDraftSafely()
})
</script>

<style scoped>
.canvas-page {
  position: relative;
  width: 100%;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  background: var(--color-bg-subtle);
  color: var(--color-text-primary);
}

.canvas-flow {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  left: 0;
  width: auto;
  height: auto;
  transition: left var(--motion-base) var(--ease-standard);
}

.canvas-page--library-open .canvas-flow { left: 276px; }
.canvas-page--inspector-open .canvas-flow { right: 328px; }

:deep(.vue-flow__pane) { cursor: grab; }
:deep(.vue-flow__pane.dragging) { cursor: grabbing; }
:deep(.vue-flow__renderer) { background: var(--color-bg-subtle); }
.canvas-page--grid-dots :deep(.vue-flow__pane) {
  background-image: radial-gradient(circle, color-mix(in srgb, var(--color-border-strong) 58%, transparent) 1px, transparent 1px);
  background-size: 20px 20px;
}
.canvas-page--grid-lines :deep(.vue-flow__pane) {
  background-image:
    linear-gradient(to right, color-mix(in srgb, var(--color-border) 42%, transparent) 1px, transparent 1px),
    linear-gradient(to bottom, color-mix(in srgb, var(--color-border) 42%, transparent) 1px, transparent 1px);
  background-size: 24px 24px;
}
.canvas-page--grid-blank :deep(.vue-flow__pane) { background-image: none; }
:deep(.vue-flow__edge-path) {
  stroke: var(--color-primary);
  stroke-width: 2.5;
  stroke-linecap: round;
  stroke-linejoin: round;
  filter: drop-shadow(0 1px 1px color-mix(in srgb, var(--color-primary) 24%, transparent));
}
:deep(.vue-flow__edge.selected .vue-flow__edge-path) {
  stroke: var(--color-accent);
  stroke-width: 3.5;
  filter: drop-shadow(0 0 3px color-mix(in srgb, var(--color-accent) 42%, transparent));
}
:deep(.vue-flow__edge-interaction) { stroke-width: 28; }
:deep(.vue-flow__connection-path) {
  stroke: var(--color-accent);
  stroke-width: 3;
  stroke-dasharray: 7 5;
}
:deep(.vue-flow__nodesselection-rect),
:deep(.vue-flow__selection) {
  border: 1px solid var(--color-primary);
  background: var(--color-primary-soft);
}

.canvas-command-bar {
  position: absolute;
  z-index: 30;
  top: 8px;
  right: 12px;
  left: 12px;
  display: grid;
  min-height: 48px;
  grid-template-columns: minmax(170px, 1fr) minmax(240px, 1.25fr) minmax(260px, 1fr);
  align-items: center;
  gap: 12px;
  padding: 5px 6px;
  border: 1px solid var(--glass-border);
  border-radius: 10px;
  background: var(--glass-bg-strong);
  box-shadow: var(--shadow-sm);
  backdrop-filter: blur(18px) saturate(112%);
  transition: left var(--motion-base) var(--ease-standard);
}

.canvas-page--library-open .canvas-command-bar { left: 288px; }

.canvas-command-bar__projects,
.canvas-command-bar__title,
.canvas-command-bar__actions {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 6px;
}

.canvas-command-bar__title { justify-content: center; }
.canvas-command-bar__actions { justify-content: flex-end; }

.canvas-brand-mark {
  display: grid;
  width: 28px;
  height: 28px;
  flex: 0 0 28px;
  place-items: center;
  border: 1px solid var(--color-primary-border);
  border-radius: 7px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  cursor: pointer;
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard),
    color var(--motion-fast) var(--ease-standard);
}

.canvas-brand-mark:hover { border-color: var(--color-primary); background: var(--color-surface-raised); }
.canvas-brand-mark:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 2px; }

.canvas-command-bar__label {
  overflow: hidden;
  color: var(--color-text-secondary);
  font-size: 12px;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.canvas-project-select,
.canvas-title-input {
  height: 36px;
  min-width: 0;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: var(--color-text-primary);
  font: inherit;
  outline: none;
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard),
    box-shadow var(--motion-fast) var(--ease-standard);
}

.canvas-project-select {
  width: min(210px, 100%);
  padding: 0 30px 0 10px;
  color: var(--color-text-secondary);
  font-size: 13px;
  font-weight: 600;
}

.canvas-title-input {
  width: min(300px, 100%);
  padding: 0 10px;
  font-size: 14px;
  font-weight: 650;
  text-align: center;
}

.canvas-project-select:hover,
.canvas-title-input:hover { background: var(--color-surface-soft); }

.canvas-project-select:focus-visible,
.canvas-title-input:focus-visible {
  border-color: var(--color-primary);
  background: var(--color-surface);
  box-shadow: 0 0 0 3px var(--color-primary-ring);
}

.canvas-icon-button,
.canvas-panel-close,
.canvas-zoom-controls button,
.canvas-tool-dock button,
.canvas-appearance-popover button {
  display: grid;
  width: 36px;
  height: 36px;
  flex: 0 0 36px;
  place-items: center;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: var(--color-text-secondary);
  cursor: pointer;
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard),
    color var(--motion-fast) var(--ease-standard);
}

.canvas-icon-button:hover,
.canvas-panel-close:hover,
.canvas-zoom-controls button:hover,
.canvas-tool-dock button:hover,
.canvas-tool-dock button.is-active,
.canvas-appearance-popover button:hover,
.canvas-appearance-popover button.is-active,
.canvas-icon-button.is-active {
  border-color: var(--color-primary-border);
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.canvas-icon-button:focus-visible,
.canvas-panel-close:focus-visible,
.canvas-zoom-controls button:focus-visible,
.canvas-tool-dock button:focus-visible,
.canvas-appearance-popover button:focus-visible,
.canvas-library-tabs button:focus-visible,
.canvas-library-summary button:focus-visible,
.canvas-library-search input:focus-visible,
.canvas-asset-item:focus-visible,
.canvas-project-item:focus-visible,
.canvas-prompt-item:focus-visible,
.canvas-node-library__item:focus-visible,
.canvas-inspector__tabs button:focus-visible,
.canvas-primary-button:focus-visible,
.canvas-secondary-button:focus-visible,
.canvas-danger-button:focus-visible,
.canvas-text-button:focus-visible,
.canvas-save-button:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}

.canvas-icon-button--danger:hover { border-color: var(--color-danger); color: var(--color-danger); }
.canvas-icon-button:disabled,
.canvas-tool-dock button:disabled,
.canvas-save-button:disabled,
.canvas-primary-button:disabled,
.canvas-secondary-button:disabled,
.canvas-node-library__item:disabled { cursor: not-allowed; opacity: 0.48; }

.canvas-command-divider { width: 1px; height: 24px; margin: 0 2px; background: var(--color-border); }

.canvas-save-button,
.canvas-primary-button,
.canvas-secondary-button,
.canvas-danger-button {
  display: inline-flex;
  min-height: 36px;
  align-items: center;
  justify-content: center;
  gap: 7px;
  padding: 0 13px;
  border: 1px solid transparent;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 650;
  cursor: pointer;
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard),
    color var(--motion-fast) var(--ease-standard),
    box-shadow var(--motion-fast) var(--ease-standard);
}

.canvas-save-button {
  min-width: 72px;
  flex: 0 0 auto;
  white-space: nowrap;
}

.canvas-save-button,
.canvas-primary-button { background: var(--color-primary); color: var(--color-text-on-primary, #fff); }
.canvas-save-button:hover,
.canvas-primary-button:hover { background: var(--color-primary-hover); }
.canvas-secondary-button { border-color: var(--color-border); background: var(--color-surface-soft); color: var(--color-text-primary); }
.canvas-secondary-button:hover { border-color: var(--color-primary-border); background: var(--color-primary-soft); color: var(--color-primary); }
.canvas-danger-button { border-color: color-mix(in srgb, var(--color-danger) 40%, var(--color-border)); background: transparent; color: var(--color-danger); }
.canvas-danger-button:hover { background: color-mix(in srgb, var(--color-danger) 10%, transparent); }
.canvas-button-full { width: 100%; min-height: 40px; }

.canvas-save-state {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 6px;
  color: var(--color-text-muted);
  font-size: 11px;
  white-space: nowrap;
}

.canvas-save-state__dot { width: 6px; height: 6px; border-radius: 50%; background: currentColor; }
.canvas-save-state--dirty { color: var(--color-warning); }
.canvas-save-state--saving { color: var(--color-info); }
.canvas-save-state--saved { color: var(--color-success); }
.canvas-save-state--error { color: var(--color-danger); }
.canvas-save-state--saving .canvas-save-state__dot { animation: canvas-save-pulse 1.1s ease-in-out infinite; }

.canvas-node-library,
.canvas-inspector {
  position: absolute;
  z-index: 20;
}

.canvas-node-library {
  top: 0;
  bottom: 0;
  left: 0;
  display: flex;
  width: 276px;
  flex-direction: column;
  overflow: hidden;
  border: 0;
  border-right: 1px solid var(--glass-border);
  border-radius: 0;
  background: var(--glass-bg-strong);
  box-shadow: var(--shadow-sm);
  backdrop-filter: blur(18px) saturate(112%);
  transform: translateX(-100%);
  transition: transform var(--motion-base) var(--ease-standard);
}

.canvas-node-library.is-open { transform: translateX(0); }

.canvas-inspector { top: 64px; right: 12px; bottom: 72px; display: flex; width: 316px; min-height: 0; flex-direction: column; gap: 8px; }
.canvas-inspector {
  opacity: 0;
  pointer-events: none;
  transform: translateX(16px);
  transition:
    opacity var(--motion-base) var(--ease-standard),
    transform var(--motion-base) var(--ease-standard);
}
.canvas-inspector.is-open { opacity: 1; pointer-events: auto; transform: translateX(0); }

.canvas-library-tabs {
  display: flex;
  min-height: 48px;
  align-items: center;
  gap: 2px;
  padding: 6px 8px 0;
  border-bottom: 1px solid var(--color-border-subtle);
}

.canvas-library-tabs > button:not(.canvas-panel-close) {
  position: relative;
  min-width: 0;
  min-height: 40px;
  flex: 1;
  padding: 0 7px;
  border: 0;
  background: transparent;
  color: var(--color-text-muted);
  font-size: 12px;
  font-weight: 650;
  cursor: pointer;
}

.canvas-library-tabs > button:not(.canvas-panel-close)::after {
  position: absolute;
  right: 8px;
  bottom: -1px;
  left: 8px;
  height: 2px;
  border-radius: 2px;
  background: transparent;
  content: '';
}

.canvas-library-tabs > button.is-active { color: var(--color-text-primary); }
.canvas-library-tabs > button.is-active::after { background: var(--color-primary); }

.canvas-library-summary {
  display: flex;
  min-height: 42px;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 12px 4px;
  color: var(--color-text-muted);
  font: 550 11px/1.3 var(--font-mono, monospace);
}

.canvas-library-summary button {
  display: inline-flex;
  min-height: 30px;
  align-items: center;
  gap: 5px;
  padding: 0 8px;
  border: 1px solid var(--color-border);
  border-radius: 7px;
  background: var(--color-surface);
  color: var(--color-text-primary);
  font: 650 11px/1 var(--font-sans, sans-serif);
  cursor: pointer;
}

.canvas-library-search {
  display: flex;
  min-height: 36px;
  align-items: center;
  gap: 7px;
  margin: 6px 10px 8px;
  padding: 0 9px;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: var(--color-surface);
  color: var(--color-text-muted);
}

.canvas-library-search:focus-within {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 3px var(--color-primary-ring);
}

.canvas-library-search input {
  width: 100%;
  min-width: 0;
  border: 0;
  background: transparent;
  color: var(--color-text-primary);
  font: inherit;
  font-size: 12px;
  outline: none;
}

.canvas-library-search input::placeholder { color: var(--color-text-muted); }

.canvas-panel-heading,
.canvas-history-header {
  display: flex;
  min-height: 58px;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.canvas-panel-heading h2,
.canvas-history-header h2,
.canvas-project-overview h2 {
  margin: 2px 0 0;
  color: var(--color-text-primary);
  font-size: 15px;
  font-weight: 700;
  line-height: 1.3;
}

.canvas-panel-eyebrow {
  color: var(--color-text-muted);
  font: 600 10px/1.2 var(--font-mono, monospace);
  text-transform: uppercase;
}

.canvas-node-library .canvas-panel-close { display: none; }
.canvas-inspector .canvas-panel-close { display: grid; }
.canvas-node-library__list,
.canvas-library-scroll { display: flex; min-height: 0; flex: 1; flex-direction: column; gap: 4px; overflow-y: auto; padding: 4px 10px 10px; }

.canvas-node-library__item {
  display: grid;
  width: 100%;
  min-height: 54px;
  grid-template-columns: 34px minmax(0, 1fr) 16px;
  align-items: center;
  gap: 9px;
  padding: 7px 8px;
  border: 1px solid transparent;
  border-radius: 7px;
  background: transparent;
  color: var(--color-text-primary);
  text-align: left;
  cursor: pointer;
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard),
    color var(--motion-fast) var(--ease-standard);
}

.canvas-node-library__item:hover { border-color: var(--color-primary-border); background: var(--color-primary-soft); }
.canvas-node-library__item > span:nth-child(2) { min-width: 0; }
.canvas-node-library__item strong { display: block; font-size: 13px; font-weight: 650; }
.canvas-node-library__item small { display: block; margin-top: 3px; overflow: hidden; color: var(--color-text-muted); font-size: 11px; line-height: 1.25; text-overflow: ellipsis; white-space: nowrap; }

.canvas-node-library__icon {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border: 1px solid var(--color-primary-border);
  border-radius: 8px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.canvas-node-library__footer {
  display: flex;
  min-height: 42px;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 0 14px;
  border-top: 1px solid var(--color-border-subtle);
  color: var(--color-text-muted);
  font: 500 10px/1 var(--font-mono, monospace);
}

.canvas-asset-item,
.canvas-project-item,
.canvas-prompt-item {
  display: grid;
  width: 100%;
  min-width: 0;
  align-items: center;
  gap: 9px;
  padding: 8px;
  border: 1px solid transparent;
  border-radius: 7px;
  background: transparent;
  color: var(--color-text-primary);
  text-align: left;
  cursor: pointer;
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard),
    color var(--motion-fast) var(--ease-standard);
}

.canvas-asset-item { grid-template-columns: 42px minmax(0, 1fr); }
.canvas-project-item { grid-template-columns: 1fr auto; }
.canvas-prompt-item { grid-template-columns: 34px minmax(0, 1fr) 16px; }
.canvas-asset-item:hover,
.canvas-project-item:hover,
.canvas-prompt-item:hover,
.canvas-project-item.is-active { border-color: var(--color-primary-border); background: var(--color-primary-soft); }

.canvas-asset-item__preview {
  display: grid;
  width: 42px;
  height: 42px;
  overflow: hidden;
  place-items: center;
  border: 1px solid var(--color-border);
  border-radius: 6px;
  background: var(--color-surface-soft);
  color: var(--color-text-muted);
}

.canvas-asset-item__preview img { width: 100%; height: 100%; object-fit: cover; }
.canvas-asset-item > span:last-child { min-width: 0; }
.canvas-asset-item strong,
.canvas-asset-item small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.canvas-asset-item strong { font-size: 12px; }
.canvas-asset-item small { margin-top: 3px; color: var(--color-text-muted); font-size: 10px; }

.canvas-project-item__title { min-width: 0; overflow: hidden; font-size: 12px; font-weight: 650; text-overflow: ellipsis; white-space: nowrap; }
.canvas-project-item__meta { color: var(--color-text-muted); font: 500 10px/1 var(--font-mono, monospace); }
.canvas-project-item__date { grid-column: 1 / -1; color: var(--color-text-muted); font-size: 10px; }
.canvas-prompt-item > span:nth-child(2) { min-width: 0; }
.canvas-prompt-item strong,
.canvas-prompt-item small,
.canvas-prompt-item em { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.canvas-prompt-item strong { font-size: 12px; }
.canvas-prompt-item small { margin-top: 3px; color: var(--color-text-secondary); font-size: 11px; font-style: normal; }
.canvas-prompt-item em { margin-top: 4px; color: var(--color-text-muted); font-size: 10px; font-style: normal; }
.canvas-library-empty { padding: 36px 12px; color: var(--color-text-muted); font-size: 12px; text-align: center; }

.canvas-inspector__tabs {
  display: grid;
  min-height: 44px;
  grid-template-columns: 1fr 1fr 36px;
  gap: 4px;
  padding: 4px;
  border: 1px solid var(--glass-border);
  border-radius: 10px;
  background: var(--glass-bg-strong);
  box-shadow: var(--shadow-sm);
  backdrop-filter: blur(18px);
}

.canvas-inspector__tabs > button:not(.canvas-panel-close) {
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: var(--color-text-muted);
  font-size: 12px;
  font-weight: 650;
  cursor: pointer;
}

.canvas-inspector__tabs > button.is-active { background: var(--color-primary-soft); color: var(--color-primary); }

.canvas-inspector__body {
  min-height: 0;
  flex: 1;
  overflow-y: auto;
  padding: 16px;
  border: 1px solid var(--color-border);
  border-radius: 12px;
  background: var(--color-surface);
  box-shadow: var(--shadow-md);
  scrollbar-gutter: stable;
}

.canvas-inspector__identity {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 10px;
  margin-bottom: 18px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.canvas-inspector__identity > div { display: flex; min-width: 0; flex-direction: column; gap: 3px; }
.canvas-inspector__identity > div span { color: var(--color-text-muted); font-size: 10px; font-weight: 600; text-transform: uppercase; }
.canvas-inspector__identity > div strong { overflow: hidden; color: var(--color-text-primary); font-size: 14px; text-overflow: ellipsis; white-space: nowrap; }

.canvas-field { display: flex; min-width: 0; flex-direction: column; gap: 6px; margin-bottom: 14px; color: var(--color-text-secondary); font-size: 12px; font-weight: 600; }
.canvas-field input,
.canvas-field textarea,
.canvas-field select {
  width: 100%;
  min-height: 38px;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: var(--color-surface-soft);
  color: var(--color-text-primary);
  padding: 8px 10px;
  font: 400 13px/1.5 inherit;
  outline: none;
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    box-shadow var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard);
}
.canvas-field textarea { min-height: 152px; resize: vertical; }
.canvas-field input:focus,
.canvas-field textarea:focus,
.canvas-field select:focus { border-color: var(--color-primary); background: var(--color-surface); box-shadow: 0 0 0 3px var(--color-primary-ring); }
.canvas-field small,
.canvas-field-help { margin: 0; color: var(--color-text-muted); font-size: 11px; font-weight: 400; line-height: 1.5; }
.canvas-field > small { text-align: right; }

.canvas-secret-input { position: relative; display: block; }
.canvas-secret-input input { padding-right: 42px; font-family: var(--font-mono, monospace); }
.canvas-secret-input button { position: absolute; top: 1px; right: 1px; display: grid; width: 36px; height: 36px; place-items: center; border: 0; border-radius: 7px; background: transparent; color: var(--color-text-muted); cursor: pointer; }
.canvas-secret-input button:hover { color: var(--color-primary); }

.canvas-inspector__section { margin: 18px 0; padding: 14px 0 2px; border-top: 1px solid var(--color-border-subtle); border-bottom: 1px solid var(--color-border-subtle); }
.canvas-inspector__section h3 { margin: 0 0 12px; color: var(--color-text-primary); font-size: 12px; font-weight: 700; }

.canvas-upload-preview {
  display: grid;
  width: 100%;
  margin-bottom: 12px;
  overflow: hidden;
  place-items: center;
  border: 1px solid var(--color-border);
  border-radius: 10px;
  background: var(--color-surface-soft);
  aspect-ratio: 4 / 3;
}
.canvas-upload-preview img { display: block; width: 100%; height: 100%; object-fit: cover; }
.canvas-upload-preview > div { display: flex; align-items: center; flex-direction: column; gap: 9px; color: var(--color-text-muted); font-size: 12px; }

.canvas-inline-error { margin: 10px 0 0; color: var(--color-danger); font-size: 12px; line-height: 1.5; overflow-wrap: anywhere; }
.canvas-danger-zone { margin-top: 20px; padding-top: 14px; border-top: 1px solid var(--color-border-subtle); }

.canvas-connection-summary,
.canvas-project-overview dl { display: grid; gap: 0; margin: 0 0 18px; }
.canvas-connection-summary > div,
.canvas-project-overview dl > div { display: flex; min-height: 42px; align-items: center; justify-content: space-between; gap: 12px; border-bottom: 1px solid var(--color-border-subtle); }
.canvas-connection-summary dt,
.canvas-project-overview dt { color: var(--color-text-muted); font-size: 12px; }
.canvas-connection-summary dd,
.canvas-project-overview dd { margin: 0; color: var(--color-text-primary); font: 600 12px/1.3 var(--font-mono, monospace); overflow-wrap: anywhere; text-align: right; }

.canvas-project-overview h2 { margin: 5px 0 18px; overflow-wrap: anywhere; }

.canvas-history-header { min-height: auto; margin: -4px 0 10px; padding: 0 0 14px; }
.canvas-history-header .canvas-secondary-button { min-height: 34px; padding: 0 10px; }
.canvas-history-state { padding: 40px 12px; color: var(--color-text-muted); font-size: 12px; text-align: center; }
.canvas-history-list { margin: 0; padding: 0; list-style: none; }
.canvas-history-list li { display: flex; min-height: 62px; align-items: center; justify-content: space-between; gap: 12px; border-bottom: 1px solid var(--color-border-subtle); }
.canvas-history-list li > div { display: flex; min-width: 0; flex-direction: column; gap: 4px; }
.canvas-history-list strong { color: var(--color-text-primary); font-size: 12px; }
.canvas-history-list span { color: var(--color-text-muted); font-size: 10px; }
.canvas-text-button { min-height: 32px; padding: 0 8px; border: 0; border-radius: 6px; background: transparent; color: var(--color-primary); font-size: 12px; font-weight: 650; cursor: pointer; }
.canvas-text-button:hover { background: var(--color-primary-soft); }

.canvas-tool-dock {
  position: absolute;
  z-index: 24;
  bottom: 14px;
  left: calc(50% + 138px);
  display: flex;
  min-height: 44px;
  max-width: calc(100% - 340px);
  align-items: center;
  gap: 2px;
  padding: 4px;
  border: 1px solid var(--glass-border);
  border-radius: 10px;
  background: var(--glass-bg-strong);
  box-shadow: var(--glass-shadow);
  transform: translateX(-50%);
  backdrop-filter: blur(18px);
  transition:
    opacity var(--motion-fast) var(--ease-standard),
    transform var(--motion-fast) var(--ease-standard),
    left var(--motion-base) var(--ease-standard);
}

.canvas-page:not(.canvas-page--library-open) .canvas-tool-dock { left: 50%; }
.canvas-tool-dock > span { width: 1px; height: 24px; margin: 0 2px; background: var(--color-border); }

.canvas-appearance-popover {
  position: absolute;
  z-index: 26;
  bottom: 68px;
  left: calc(50% + 276px);
  width: 236px;
  padding: 12px;
  border: 1px solid var(--glass-border);
  border-radius: 10px;
  background: var(--glass-bg-strong);
  box-shadow: var(--shadow-overlay);
  transform: translateX(-50%);
  backdrop-filter: blur(18px);
}

.canvas-page:not(.canvas-page--library-open) .canvas-appearance-popover { left: calc(50% + 138px); }
.canvas-appearance-popover > div:first-child { display: flex; min-height: 30px; align-items: center; justify-content: space-between; gap: 8px; }
.canvas-appearance-popover > div:first-child strong { font-size: 13px; }
.canvas-appearance-popover > span { display: block; margin: 8px 0 6px; color: var(--color-text-muted); font-size: 11px; font-weight: 600; }
.canvas-appearance-popover > div:first-child button { width: 30px; height: 30px; flex-basis: 30px; }
.canvas-grid-options { display: grid; grid-template-columns: repeat(3, 1fr); gap: 4px; padding: 3px; border-radius: 8px; background: var(--color-surface-soft); }
.canvas-grid-options button { width: auto; height: 32px; padding: 0 7px; font-size: 11px; }

.canvas-zoom-controls {
  position: absolute;
  z-index: 22;
  bottom: 16px;
  left: 290px;
  display: flex;
  min-height: 44px;
  align-items: center;
  gap: 3px;
  padding: 4px;
  border: 1px solid var(--glass-border);
  border-radius: 10px;
  background: var(--glass-bg-strong);
  box-shadow: var(--glass-shadow);
  transform: none;
  backdrop-filter: blur(18px);
  transition:
    opacity var(--motion-fast) var(--ease-standard),
    transform var(--motion-fast) var(--ease-standard);
}
.canvas-zoom-controls .canvas-zoom-value { width: 54px; color: var(--color-text-primary); font: 600 11px/1 var(--font-mono, monospace); }
.canvas-page:not(.canvas-page--library-open) .canvas-zoom-controls { left: 14px; }
.canvas-zoom-range {
  width: 76px;
  height: 28px;
  accent-color: var(--color-primary);
  cursor: pointer;
}

.canvas-empty-state {
  position: absolute;
  z-index: 10;
  top: 50%;
  left: 50%;
  width: min(440px, calc(100% - 48px));
  color: var(--color-text-primary);
  text-align: center;
  transform: translate(-50%, -50%);
}
.canvas-page--library-open .canvas-empty-state { left: calc(50% + 138px); }
.canvas-empty-state__icon { display: grid; width: 54px; height: 54px; margin: 0 auto 16px; place-items: center; border: 1px solid var(--color-primary-border); border-radius: 12px; background: var(--color-primary-soft); color: var(--color-primary); }
.canvas-empty-state h1 { margin: 0; font-size: 20px; font-weight: 700; }
.canvas-empty-state p { max-width: 420px; margin: 8px auto 18px; color: var(--color-text-secondary); font-size: 13px; line-height: 1.6; }
.canvas-empty-state > div { display: flex; justify-content: center; gap: 8px; }

.canvas-loading {
  position: absolute;
  z-index: 60;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  background: var(--color-bg);
  color: var(--color-text-secondary);
  font-size: 13px;
}
.canvas-loading__spinner { width: 18px; height: 18px; border: 2px solid var(--color-border); border-top-color: var(--color-primary); border-radius: 50%; animation: canvas-spin 0.8s linear infinite; }

.canvas-drop-overlay {
  position: absolute;
  z-index: 50;
  inset: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  gap: 12px;
  border: 2px dashed var(--color-primary);
  border-radius: 14px;
  background: color-mix(in srgb, var(--color-primary-soft) 86%, transparent);
  color: var(--color-primary);
  pointer-events: none;
}

.canvas-message-banner {
  position: absolute;
  z-index: 70;
  right: 16px;
  bottom: 16px;
  display: flex;
  width: min(520px, calc(100% - 32px));
  min-height: 44px;
  align-items: center;
  gap: 9px;
  padding: 8px 10px;
  border: 1px solid color-mix(in srgb, var(--color-danger) 42%, var(--glass-border));
  border-radius: 10px;
  background: var(--glass-bg-strong);
  box-shadow: var(--shadow-overlay);
  color: var(--color-danger);
  backdrop-filter: blur(18px);
}
.canvas-message-banner--success {
  border-color: color-mix(in srgb, var(--color-success) 42%, var(--glass-border));
  color: var(--color-success);
}
.canvas-message-banner span { min-width: 0; flex: 1; font-size: 12px; line-height: 1.45; overflow-wrap: anywhere; }
.canvas-message-banner button { min-height: 30px; border: 0; border-radius: 6px; background: transparent; color: inherit; font-size: 12px; font-weight: 650; cursor: pointer; }

.canvas-mobile-panel-button { display: none; }

@keyframes canvas-spin { to { transform: rotate(360deg); } }
@keyframes canvas-save-pulse { 0%, 100% { opacity: 0.4; } 50% { opacity: 1; } }

@media (max-width: 1500px) {
  .canvas-page--library-open .canvas-action-optional:not(.canvas-icon-button--danger),
  .canvas-page--library-open .canvas-command-divider { display: none; }
}

@media (max-width: 1180px) {
  .canvas-command-bar { grid-template-columns: minmax(150px, 0.8fr) minmax(180px, 1fr) auto; }
  .canvas-save-state { display: none; }
  .canvas-inspector { width: 286px; }
}

@media (max-width: 1180px) {
  .canvas-flow,
  .canvas-page--library-open .canvas-flow,
  .canvas-page--inspector-open .canvas-flow { inset: 0; }
  .canvas-page--library-open .canvas-command-bar { left: 12px; }
  .canvas-page--library-open .canvas-empty-state { left: 50%; }
  .canvas-zoom-controls,
  .canvas-page:not(.canvas-page--library-open) .canvas-zoom-controls { left: 12px; }
  .canvas-tool-dock,
  .canvas-page:not(.canvas-page--library-open) .canvas-tool-dock { left: 50%; max-width: calc(100% - 24px); }
  .canvas-appearance-popover,
  .canvas-page:not(.canvas-page--library-open) .canvas-appearance-popover { right: 12px; left: auto; transform: none; }
  .canvas-command-bar { grid-template-columns: minmax(180px, 1fr) minmax(160px, 1fr) auto; }
  .canvas-node-library {
    top: 64px;
    bottom: 8px;
    left: 8px;
    width: min(276px, calc(100% - 16px));
    border: 1px solid var(--glass-border);
    border-radius: 10px;
    opacity: 0;
    pointer-events: none;
    transform: translateX(calc(-100% - 16px));
    transition:
      opacity var(--motion-base) var(--ease-standard),
      transform var(--motion-base) var(--ease-standard);
  }
  .canvas-node-library.is-open { opacity: 1; pointer-events: auto; transform: translateX(0); }
  .canvas-node-library .canvas-panel-close { display: grid; }
  .canvas-mobile-panel-button { display: grid; }
  .canvas-page--panel-open .canvas-zoom-controls,
  .canvas-page--panel-open .canvas-tool-dock,
  .canvas-page--panel-open .canvas-appearance-popover {
    opacity: 0;
    pointer-events: none;
  }
  .canvas-page--panel-open :deep(.canvas-minimap) { display: none; }
  .canvas-page--panel-open .canvas-zoom-controls,
  .canvas-page--panel-open .canvas-appearance-popover { transform: translateY(8px); }
  .canvas-page--panel-open .canvas-tool-dock { transform: translate(-50%, 8px); }
}

@media (max-width: 760px) {
  .canvas-command-bar {
    top: 8px;
    right: 8px;
    left: 8px;
    min-height: 48px;
    grid-template-columns: minmax(116px, 1fr) auto;
    gap: 6px;
    padding: 5px;
  }
  .canvas-command-bar__projects .canvas-icon-button--danger,
  .canvas-command-bar__title,
  .canvas-action-optional,
  .canvas-command-divider { display: none; }
  .canvas-project-select { height: 36px; width: 100%; padding-left: 8px; }
  .canvas-command-bar__actions { grid-column: 2; }
  .canvas-save-button { min-width: 72px; padding: 0 10px; }
  .canvas-node-library,
  .canvas-inspector {
    top: 64px;
    right: 8px;
    bottom: 8px;
    left: 8px;
    width: auto;
    max-height: none;
  }
  .canvas-node-library { transform: translateY(16px); }
  .canvas-node-library.is-open { transform: translateY(0); }
  .canvas-inspector {
    transform: translateY(16px);
  }
  .canvas-inspector.is-open { opacity: 1; pointer-events: auto; transform: translateY(0); }
  .canvas-inspector__tabs { flex: 0 0 auto; }
  .canvas-inspector__body { padding: 16px; }
  .canvas-zoom-controls { bottom: 10px; }
  .canvas-page--panel-open .canvas-zoom-controls {
    opacity: 0;
    pointer-events: none;
    transform: translate(-50%, 8px);
  }
  .canvas-tool-dock,
  .canvas-page:not(.canvas-page--library-open) .canvas-tool-dock {
    right: 8px;
    bottom: 8px;
    left: 8px;
    max-width: none;
    overflow-x: auto;
    transform: none;
    scrollbar-width: none;
  }
  .canvas-tool-dock::-webkit-scrollbar { display: none; }
  .canvas-tool-dock button { flex: 0 0 36px; }
  .canvas-zoom-controls { right: auto; bottom: 60px; left: 8px; }
  .canvas-zoom-range { width: 62px; }
  .canvas-page--panel-open .canvas-tool-dock,
  .canvas-page--panel-open .canvas-appearance-popover {
    opacity: 0;
    pointer-events: none;
    transform: translateY(8px);
  }
  .canvas-empty-state { top: 46%; }
  .canvas-empty-state p { font-size: 12px; }
  .canvas-empty-state > div { align-items: stretch; flex-direction: column; }
  .canvas-primary-button,
  .canvas-secondary-button { min-height: 44px; }
  .canvas-message-banner { right: 8px; bottom: 62px; width: calc(100% - 16px); }
}

@media (max-width: 420px) {
  .canvas-command-bar__projects { gap: 3px; }
  .canvas-command-bar__projects .canvas-icon-button { width: 34px; height: 34px; flex-basis: 34px; }
  .canvas-command-bar__actions { gap: 3px; }
  .canvas-command-bar__actions .canvas-icon-button { width: 34px; height: 34px; flex-basis: 34px; }
  .canvas-save-button { min-width: 72px; font-size: 12px; }
  .canvas-project-select { font-size: 12px; }
}

@media (prefers-reduced-motion: reduce) {
  .canvas-page *,
  .canvas-page *::before,
  .canvas-page *::after {
    scroll-behavior: auto !important;
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
  }
}

.canvas-selection-tools {
  position: absolute;
  z-index: 28;
  top: 76px;
  left: 50%;
  display: flex;
  min-height: 42px;
  align-items: center;
  gap: 4px;
  padding: 4px 6px 4px 12px;
  border: 1px solid var(--glass-border);
  border-radius: 9px;
  background: var(--glass-bg-strong);
  box-shadow: var(--shadow-md);
  transform: translateX(-50%);
  backdrop-filter: blur(14px);
}

.canvas-selection-tools > span { margin-right: 4px; color: var(--color-text-muted); font-size: 11px; white-space: nowrap; }
.canvas-selection-tools button, .canvas-context-menu button { display: inline-flex; min-height: 34px; align-items: center; gap: 7px; border: 0; border-radius: 7px; background: transparent; color: var(--color-text-secondary); font-size: 12px; cursor: pointer; }
.canvas-selection-tools button { padding: 0 9px; }
.canvas-selection-tools button:hover, .canvas-context-menu button:hover { background: var(--color-primary-soft); color: var(--color-primary); }
.canvas-context-menu { position: fixed; z-index: 90; display: grid; min-width: 190px; max-height: min(420px, calc(100vh - 24px)); overflow-y: auto; padding: 5px; border: 1px solid var(--glass-border); border-radius: 9px; background: var(--glass-bg-strong); box-shadow: var(--shadow-overlay); backdrop-filter: blur(16px); }
.canvas-context-menu__label { padding: 8px 10px 6px; color: var(--color-text-muted); font-size: 11px; font-weight: 650; }
.canvas-context-menu__divider { height: 1px; margin: 4px 6px; background: var(--color-border-subtle); }
.canvas-context-menu button { width: 100%; justify-content: flex-start; padding: 0 10px; text-align: left; }
.canvas-context-menu button.is-danger { color: var(--color-danger); }
.canvas-context-menu button.is-danger:hover { background: color-mix(in srgb, var(--color-danger) 10%, transparent); color: var(--color-danger); }
.canvas-option-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 10px; }
.canvas-upload-preview video { display: block; width: 100%; height: 100%; object-fit: contain; background: var(--color-bg-deep); }
.canvas-upload-preview audio { width: calc(100% - 24px); }
.canvas-upload-preview--audio { min-height: 112px; aspect-ratio: auto; }
.canvas-field output { float: right; color: var(--color-text-muted); font: 500 11px/1 var(--font-mono, monospace); }
.canvas-page--library-open :deep(.canvas-minimap) { left: 316px; }
.canvas-page--inspector-open :deep(.canvas-minimap) { max-width: calc(100% - 360px); }

@media (max-width: 940px) {
  .canvas-page--library-open :deep(.canvas-minimap) { left: 12px; }
}

@media (max-width: 620px) {
  .canvas-selection-tools { top: 68px; max-width: calc(100% - 20px); overflow-x: auto; }
  .canvas-option-grid { grid-template-columns: 1fr; }
  .canvas-context-menu { min-width: 176px; }
}
</style>
