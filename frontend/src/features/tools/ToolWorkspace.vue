<template>
  <section class="tool-workspace" :aria-labelledby="`tool-title-${tool}`">
    <header class="tool-workspace__header">
      <div class="tool-workspace__heading">
        <span class="tool-workspace__eyebrow">{{ label('tools.workspace.eyebrow', 'TOOLBOX') }}</span>
        <h1 :id="`tool-title-${tool}`">{{ label(definition.titleKey, tool) }}</h1>
        <p>{{ label(definition.descriptionKey, label('tools.workspace.localOnly', 'Runs locally in this browser.')) }}</p>
        <div class="tool-workspace__badges" aria-label="tool properties">
          <span><Icon name="shield" size="sm" aria-hidden="true" />{{ categoryLabel }}</span>
          <span><Icon name="terminal" size="sm" aria-hidden="true" />{{ label('tools.workspace.localBadge', 'Local') }}</span>
          <span><Icon name="lock" size="sm" aria-hidden="true" />{{ label('tools.workspace.privacyBadge', 'Private') }}</span>
        </div>
      </div>

      <aside class="tool-workspace__context">
        <div class="tool-workspace__context-art" aria-hidden="true">
          <span class="tool-workspace__context-card tool-workspace__context-card--back"></span>
          <span class="tool-workspace__context-card tool-workspace__context-card--front"><Icon :name="definition.icon" size="xl" /></span>
        </div>
        <div class="tool-workspace__context-copy">
          <strong>{{ label('tools.workspace.contextTitle', 'Local tool workspace') }}</strong>
          <p>{{ label('tools.categoryDescription.' + definition.category, categoryLabel) }}</p>
          <ul>
            <li><Icon name="checkCircle" size="sm" aria-hidden="true" />{{ label('tools.workspace.contextPointLocal', 'Runs in this browser') }}</li>
            <li><Icon name="checkCircle" size="sm" aria-hidden="true" />{{ label('tools.workspace.contextPointPrivate', 'Inputs are not uploaded') }}</li>
            <li><Icon name="checkCircle" size="sm" aria-hidden="true" />{{ label('tools.workspace.contextPointReusable', 'Reusable engineering utility') }}</li>
          </ul>
        </div>
      </aside>
    </header>
    <p v-if="noticeKey" class="tool-workspace__notice" role="note">{{ label(noticeKey, '') }}</p>

    <div class="tool-workspace__grid">
      <form class="tool-panel tool-panel--input" @submit.prevent="runTool">
        <div class="tool-panel__header">
          <div class="tool-panel__title">
            <span class="tool-panel__step">1</span>
            <div>
              <h2>{{ label('tools.workspace.input', 'Input') }}</h2>
              <p>{{ label('tools.workspace.inputHint', 'Configure the values used by this tool.') }}</p>
            </div>
          </div>
          <button type="button" class="tool-button tool-button--ghost" @click="resetTool"><Icon name="refresh" size="sm" aria-hidden="true" />{{ label('tools.workspace.reset', 'Reset') }}</button>
        </div>

        <template v-if="tool === 'totp'">
          <label class="tool-field"><span>{{ label('tools.workspace.secretOrUri', 'Secret or otpauth URI') }}</span><span class="tool-input-wrap"><input v-model="input" :type="showSecret ? 'text' : 'password'" autocomplete="off" spellcheck="false" class="tool-input tool-input--with-action" /><button type="button" class="tool-input-action" :aria-label="label(showSecret ? 'tools.workspace.hideValue' : 'tools.workspace.showValue', showSecret ? 'Hide value' : 'Show value')" :title="label(showSecret ? 'tools.workspace.hideValue' : 'tools.workspace.showValue', showSecret ? 'Hide value' : 'Show value')" @click="showSecret = !showSecret"><Icon :name="showSecret ? 'eyeOff' : 'eye'" size="sm" aria-hidden="true" /></button></span></label>
          <p class="tool-help">{{ label('tools.workspace.totpHint', 'Base32 secrets and otpauth://totp/... URIs are processed locally.') }}</p>
        </template>

        <template v-else-if="tool === 'password' || tool === 'random-string'">
          <label class="tool-field"><span>{{ label('tools.workspace.length', 'Length') }}</span><input v-model.number="options.length" type="number" min="1" max="8192" class="tool-input" /></label>
          <div class="tool-checks">
            <label><input v-model="options.uppercase" type="checkbox" /> {{ label('tools.workspace.uppercase', 'Uppercase') }}</label>
            <label><input v-model="options.lowercase" type="checkbox" /> {{ label('tools.workspace.lowercase', 'Lowercase') }}</label>
            <label><input v-model="options.numbers" type="checkbox" /> {{ label('tools.workspace.numbers', 'Numbers') }}</label>
            <label><input v-model="options.symbols" type="checkbox" /> {{ label('tools.workspace.symbols', 'Symbols') }}</label>
          </div>
          <label class="tool-field"><span>{{ label('tools.workspace.customCharacters', 'Custom characters') }}</span><input v-model="options.customCharacters" type="text" class="tool-input" /></label>
          <label class="tool-field"><span>{{ label('tools.workspace.excludeCharacters', 'Exclude characters') }}</span><input v-model="options.excludeCharacters" type="text" class="tool-input" /></label>
          <label class="tool-field"><span>{{ label('tools.workspace.batch', 'Batch') }}</span><input v-model.number="batch" type="number" min="1" max="20" class="tool-input" /></label>
          <p v-if="randomAlphabetSize > 0" class="tool-help tool-entropy">{{ label('tools.workspace.entropy', 'Estimated entropy') }}: {{ randomEntropyBits.toFixed(1) }} {{ label('tools.workspace.bits', 'bits') }} / {{ label('tools.workspace.strength', 'Strength') }}: {{ randomStrength }}</p>
        </template>

        <template v-else-if="tool === 'uuid'">
          <label class="tool-field"><span>{{ label('tools.workspace.variant', 'Variant') }}</span><select v-model="uuidVariant" class="tool-input"><option value="v4">UUID v4</option><option value="v7">UUID v7</option></select></label>
          <label class="tool-field"><span>{{ label('tools.workspace.batch', 'Batch') }}</span><input v-model.number="batch" type="number" min="1" max="100" class="tool-input" /></label>
        </template>

        <template v-else-if="tool === 'json'">
          <label class="tool-field"><span>{{ label('tools.workspace.operation', 'Operation') }}</span><select v-model="operation" class="tool-input"><option value="format">{{ label('tools.workspace.format', 'Format') }}</option><option value="minify">{{ label('tools.workspace.minify', 'Minify') }}</option><option value="validate">{{ label('tools.workspace.validate', 'Validate') }}</option></select></label>
          <label v-if="operation === 'format'" class="tool-field"><span>{{ label('tools.workspace.indent', 'Indent') }}</span><input v-model.number="indent" type="number" min="0" max="10" class="tool-input" /></label>
          <label class="tool-field"><span>{{ label('tools.workspace.json', 'JSON') }}</span><textarea v-model="input" rows="12" class="tool-input tool-input--mono" /></label>
        </template>

        <template v-else-if="tool === 'base64'">
          <label class="tool-field"><span>{{ label('tools.workspace.operation', 'Operation') }}</span><select v-model="operation" class="tool-input"><option value="encode">{{ label('tools.workspace.encode', 'Encode') }}</option><option value="decode">{{ label('tools.workspace.decode', 'Decode') }}</option></select></label>
          <label class="tool-check"><input v-model="urlSafe" type="checkbox" /> {{ label('tools.workspace.base64Url', 'Base64URL alphabet') }}</label>
          <label class="tool-field"><span>{{ label('tools.workspace.text', 'Text') }}</span><textarea v-model="input" rows="10" class="tool-input tool-input--mono" /></label>
        </template>

        <template v-else-if="tool === 'url'">
          <label class="tool-field"><span>{{ label('tools.workspace.operation', 'Operation') }}</span><select v-model="operation" class="tool-input"><option value="encode">{{ label('tools.workspace.encode', 'Encode') }}</option><option value="decode">{{ label('tools.workspace.decode', 'Decode') }}</option></select></label>
          <label class="tool-field"><span>{{ label('tools.workspace.text', 'Text') }}</span><textarea v-model="input" rows="10" class="tool-input tool-input--mono" /></label>
        </template>

        <template v-else-if="tool === 'timestamp'">
          <label class="tool-field"><span>{{ label('tools.workspace.operation', 'Operation') }}</span><select v-model="operation" class="tool-input"><option value="timestampToDate">{{ label('tools.workspace.timestampToDate', 'Timestamp → date') }}</option><option value="dateToTimestamp">{{ label('tools.workspace.dateToTimestamp', 'Date → timestamp') }}</option></select></label>
          <label class="tool-field"><span>{{ operation === 'timestampToDate' ? label('tools.workspace.timestamp', 'Timestamp') : label('tools.workspace.dateTime', 'Date/time') }}</span><input v-model="input" :type="operation === 'timestampToDate' ? 'text' : 'datetime-local'" class="tool-input" /></label>
          <label v-if="operation === 'timestampToDate'" class="tool-field"><span>{{ label('tools.workspace.unit', 'Unit') }}</span><select v-model="timestampUnit" class="tool-input"><option value="auto">{{ label('tools.workspace.autoDetect', 'Auto detect') }}</option><option value="seconds">{{ label('tools.workspace.seconds', 'Seconds') }}</option><option value="milliseconds">{{ label('tools.workspace.milliseconds', 'Milliseconds') }}</option></select></label>
          <button type="button" class="tool-button tool-button--ghost" @click="showCurrentTimestamp"><Icon name="clock" size="sm" aria-hidden="true" />{{ label('tools.workspace.currentTimestamp', 'Use current timestamp') }}</button>
        </template>

        <template v-else-if="tool === 'hash'">
          <label class="tool-field"><span>{{ label('tools.workspace.algorithm', 'Algorithm') }}</span><select v-model="algorithm" class="tool-input"><option>SHA-1</option><option>SHA-256</option><option>SHA-384</option><option>SHA-512</option></select></label>
          <label class="tool-field"><span>{{ label('tools.workspace.text', 'Text') }}</span><textarea v-model="input" rows="10" class="tool-input tool-input--mono" /></label>
        </template>

        <template v-else-if="tool === 'hmac'">
          <label class="tool-field"><span>{{ label('tools.workspace.algorithm', 'Algorithm') }}</span><select v-model="algorithm" class="tool-input"><option>SHA-256</option><option>SHA-384</option><option>SHA-512</option></select></label>
          <label class="tool-field"><span>{{ label('tools.workspace.secretLabel', 'Secret') }}</span><span class="tool-input-wrap"><input v-model="secret" :type="showSecret ? 'text' : 'password'" autocomplete="off" class="tool-input tool-input--with-action" /><button type="button" class="tool-input-action" :aria-label="label(showSecret ? 'tools.workspace.hideValue' : 'tools.workspace.showValue', showSecret ? 'Hide value' : 'Show value')" :title="label(showSecret ? 'tools.workspace.hideValue' : 'tools.workspace.showValue', showSecret ? 'Hide value' : 'Show value')" @click="showSecret = !showSecret"><Icon :name="showSecret ? 'eyeOff' : 'eye'" size="sm" aria-hidden="true" /></button></span></label>
          <label class="tool-field"><span>{{ label('tools.workspace.message', 'Message') }}</span><textarea v-model="input" rows="10" class="tool-input tool-input--mono" /></label>
        </template>

        <template v-else-if="tool === 'regex'">
          <label class="tool-field"><span>{{ label('tools.workspace.pattern', 'Pattern') }}</span><input v-model="pattern" type="text" class="tool-input tool-input--mono" /></label>
          <label class="tool-field"><span>{{ label('tools.workspace.flags', 'Flags') }}</span><input v-model="flags" type="text" inputmode="text" class="tool-input tool-input--mono" /></label>
          <label class="tool-field"><span>{{ label('tools.workspace.testText', 'Test text') }}</span><textarea v-model="input" rows="6" class="tool-input tool-input--mono" /></label>
        </template>

        <template v-else-if="tool === 'http-status'">
          <label class="tool-field"><span>{{ label('tools.workspace.statusSearch', 'Search code or name') }}</span><input v-model="input" type="search" class="tool-input" /></label>
        </template>

        <template v-else-if="tool === 'user-agent'">
          <label class="tool-field"><span>{{ label('tools.workspace.userAgent', 'User-Agent') }}</span><textarea v-model="input" rows="6" class="tool-input tool-input--mono" /></label>
        </template>

        <template v-else-if="tool === 'ip-cidr'">
          <label class="tool-field"><span>{{ label('tools.workspace.cidr', 'IPv4 / IPv6 CIDR') }}</span><input v-model="input" type="text" placeholder="192.168.1.0/24" class="tool-input tool-input--mono" /></label>
        </template>

        <template v-else-if="tool === 'api-builder'">
          <label class="tool-field"><span>{{ label('tools.workspace.url', 'URL') }}</span><input v-model="api.url" type="url" required placeholder="https://example.test/api" class="tool-input tool-input--mono" /></label>
          <label class="tool-field"><span>{{ label('tools.workspace.method', 'Method') }}</span><select v-model="api.method" class="tool-input"><option v-for="method in methods" :key="method">{{ method }}</option></select></label>
          <label class="tool-field"><span>{{ label('tools.workspace.query', 'Query parameters (JSON)') }}</span><textarea v-model="api.query" rows="3" class="tool-input tool-input--mono" placeholder="{ &quot;page&quot;: &quot;1&quot; }" /></label>
          <label class="tool-field"><span>{{ label('tools.workspace.headers', 'Headers (JSON)') }}</span><textarea v-model="api.headers" rows="4" class="tool-input tool-input--mono" /></label>
          <label class="tool-field"><span>{{ label('tools.workspace.authorization', 'Authorization') }}</span><select v-model="api.authType" class="tool-input"><option value="none">{{ label('tools.workspace.authNone', 'None') }}</option><option value="bearer">{{ label('tools.workspace.authBearer', 'Bearer token') }}</option><option value="basic">{{ label('tools.workspace.authBasic', 'Basic authentication') }}</option><option value="api-key">{{ label('tools.workspace.authApiKey', 'API key') }}</option></select></label>
          <label v-if="api.authType !== 'none'" class="tool-field"><span>{{ label('tools.workspace.authValue', 'Authorization value') }}</span><span class="tool-input-wrap"><input v-model="api.authValue" :type="showAuthValue ? 'text' : 'password'" autocomplete="off" class="tool-input tool-input--mono tool-input--with-action" /><button type="button" class="tool-input-action" :aria-label="label(showAuthValue ? 'tools.workspace.hideValue' : 'tools.workspace.showValue', showAuthValue ? 'Hide value' : 'Show value')" :title="label(showAuthValue ? 'tools.workspace.hideValue' : 'tools.workspace.showValue', showAuthValue ? 'Hide value' : 'Show value')" @click="showAuthValue = !showAuthValue"><Icon :name="showAuthValue ? 'eyeOff' : 'eye'" size="sm" aria-hidden="true" /></button></span></label>
          <label v-if="api.authType === 'basic'" class="tool-field"><span>{{ label('tools.workspace.authUsername', 'Username') }}</span><input v-model="api.authUsername" type="text" autocomplete="off" class="tool-input" /></label>
          <label v-if="api.authType === 'api-key'" class="tool-field"><span>{{ label('tools.workspace.authHeader', 'API key header') }}</span><input v-model="api.authHeaderName" type="text" class="tool-input" /></label>
          <label class="tool-field"><span>{{ label('tools.workspace.bodyType', 'Body type') }}</span><select v-model="api.bodyType" class="tool-input"><option value="json">{{ label('tools.workspace.bodyJson', 'JSON') }}</option><option value="form">{{ label('tools.workspace.bodyForm', 'Form data') }}</option><option value="text">{{ label('tools.workspace.bodyText', 'Raw text') }}</option></select></label>
          <label class="tool-field"><span>{{ label('tools.workspace.body', 'Body') }}</span><textarea v-model="api.body" rows="5" class="tool-input tool-input--mono" /></label>
        </template>

        <template v-else-if="tool === 'curl-converter'">
          <label class="tool-field"><span>{{ label('tools.workspace.curlCommand', 'cURL command') }}</span><textarea v-model="input" rows="9" class="tool-input tool-input--mono" /></label>
        </template>

        <template v-else-if="tool === 'json-to-struct'">
          <label class="tool-field"><span>{{ label('tools.workspace.rootName', 'Root name') }}</span><input v-model="rootName" type="text" class="tool-input" /></label>
          <label class="tool-field"><span>{{ label('tools.workspace.json', 'JSON') }}</span><textarea v-model="input" rows="10" class="tool-input tool-input--mono" /></label>
          <label class="tool-field"><span>{{ label('tools.workspace.language', 'Language') }}</span><select v-model="language" class="tool-input"><option value="typescript">{{ label('tools.workspace.typescript', 'TypeScript') }}</option><option value="go">{{ label('tools.workspace.go', 'Go') }}</option><option value="python">{{ label('tools.workspace.python', 'Python') }}</option></select></label>
        </template>

        <template v-else-if="tool === 'cron'">
          <label class="tool-field"><span>{{ label('tools.workspace.cronExpression', '5-field expression') }}</span><input v-model="input" type="text" placeholder="*/15 * * * *" class="tool-input tool-input--mono" /></label>
          <label class="tool-field"><span>{{ label('tools.workspace.upcomingRuns', 'Upcoming runs') }}</span><input v-model.number="batch" type="number" min="1" max="20" class="tool-input" /></label>
        </template>

        <template v-else-if="tool === 'diff'">
          <label class="tool-field"><span>{{ label('tools.workspace.original', 'Original') }}</span><textarea v-model="original" rows="7" class="tool-input tool-input--mono" /></label>
          <label class="tool-field"><span>{{ label('tools.workspace.modified', 'Modified') }}</span><textarea v-model="modified" rows="7" class="tool-input tool-input--mono" /></label>
          <label class="tool-field"><span>{{ label('tools.workspace.diffMode', 'Diff mode') }}</span><select v-model="diffMode" class="tool-input"><option value="lines">{{ label('tools.workspace.lineDiff', 'Lines') }}</option><option value="characters">{{ label('tools.workspace.characterDiff', 'Characters') }}</option></select></label>
          <div class="tool-checks"><label><input v-model="ignoreCase" type="checkbox" /> {{ label('tools.workspace.ignoreCase', 'Ignore case') }}</label><label><input v-model="ignoreWhitespace" type="checkbox" /> {{ label('tools.workspace.ignoreWhitespace', 'Ignore whitespace') }}</label></div>
        </template>

        <template v-else-if="tool === 'markdown'">
          <label class="tool-field"><span>{{ label('tools.workspace.markdown', 'Markdown') }}</span><textarea v-model="input" rows="12" class="tool-input tool-input--mono" /></label>
        </template>

        <template v-else-if="tool === 'qrcode'">
          <label class="tool-field"><span>{{ label('tools.workspace.textOrUrl', 'Text or URL') }}</span><textarea v-model="input" rows="5" class="tool-input" /></label>
          <div class="tool-row"><label class="tool-field"><span>{{ label('tools.workspace.size', 'Size') }}</span><input v-model.number="qrSize" type="number" min="64" max="2048" class="tool-input" /></label><label class="tool-field"><span>{{ label('tools.workspace.errorCorrection', 'Error correction') }}</span><select v-model="qrLevel" class="tool-input"><option>L</option><option>M</option><option>Q</option><option>H</option></select></label></div>
        </template>

        <div v-else-if="tool === 'jwt'" class="tool-field"><span>{{ label('tools.workspace.jwt', 'JWT') }}</span><span class="tool-input-wrap"><input v-model="input" :type="showJwt ? 'text' : 'password'" autocomplete="off" spellcheck="false" class="tool-input tool-input--mono tool-input--with-action" /><button type="button" class="tool-input-action" :aria-label="label(showJwt ? 'tools.workspace.hideValue' : 'tools.workspace.showValue', showJwt ? 'Hide value' : 'Show value')" :title="label(showJwt ? 'tools.workspace.hideValue' : 'tools.workspace.showValue', showJwt ? 'Hide value' : 'Show value')" @click="showJwt = !showJwt"><Icon :name="showJwt ? 'eyeOff' : 'eye'" size="sm" aria-hidden="true" /></button></span></div>
        <button type="submit" class="tool-button tool-button--primary" :disabled="isRunning"><span v-if="isRunning" class="tool-spinner" aria-hidden="true" /><Icon v-else name="play" size="sm" aria-hidden="true" />{{ isRunning ? label('tools.workspace.running', 'Processing...') : actionLabel }}</button>
      </form>

      <div class="tool-workspace__side">
      <section class="tool-panel tool-panel--output">
        <div class="tool-panel__header">
          <div class="tool-panel__title">
            <span class="tool-panel__step">2</span>
            <div>
              <h2>{{ label('tools.workspace.output', 'Output') }}</h2>
              <p>{{ label('tools.workspace.outputHint', 'Results from the local operation appear here.') }}</p>
            </div>
          </div>
          <div class="tool-panel__actions">
            <button v-if="outputText || outputHtml || liveMarkdownHtml" type="button" class="tool-button tool-button--ghost" @click="copyOutput"><Icon name="copy" size="sm" aria-hidden="true" />{{ copied ? label('tools.workspace.copied', 'Copied') : label('tools.workspace.copy', 'Copy') }}</button>
            <button v-if="outputText || outputHtml || liveMarkdownHtml" type="button" class="tool-button tool-button--ghost" @click="downloadOutput"><Icon name="download" size="sm" aria-hidden="true" />{{ label('tools.workspace.download', 'Download') }}</button>
          </div>
        </div>
        <p v-if="error" class="tool-error" role="alert">{{ error }}</p>
        <p v-if="isRunning" class="tool-help" role="status" aria-live="polite">{{ label('tools.workspace.running', 'Processing...') }}</p>
        <div v-if="totpDetails" class="tool-totp-result" aria-live="polite">
          <strong class="tool-totp-result__code">{{ totpDetails.code }}</strong>
          <span>{{ label('tools.workspace.remaining', 'Remaining') }}: {{ totpDetails.remainingSeconds }} {{ label('tools.workspace.seconds', 'seconds') }}</span>
          <span v-if="totpDetails.issuer">{{ label('tools.workspace.issuer', 'Issuer') }}: {{ totpDetails.issuer }}</span>
          <span v-if="totpDetails.account">{{ label('tools.workspace.account', 'Account') }}: {{ totpDetails.account }}</span>
          <span>{{ totpDetails.algorithm }} · {{ totpDetails.digits }} {{ label('tools.workspace.digits', 'digits') }} · {{ totpDetails.period }}{{ label('tools.workspace.period', 's period') }}</span>
        </div>
        <pre v-if="outputText && !totpDetails" class="tool-output" :class="{ 'tool-output--mono': true }">{{ outputText }}</pre>
        <div v-if="outputHtml || liveMarkdownHtml" class="tool-markdown" v-html="liveMarkdownHtml || outputHtml" />
        <img v-if="qrDataUrl" :src="qrDataUrl" :alt="label('tools.workspace.generatedQr', 'Generated QR code')" class="tool-qr" />
        <div v-if="qrDataUrl" class="tool-panel__actions tool-panel__actions--qr">
          <button type="button" class="tool-button tool-button--ghost" @click="downloadQr('png')"><Icon name="download" size="sm" aria-hidden="true" />{{ label('tools.workspace.downloadPng', 'Download PNG') }}</button>
          <button type="button" class="tool-button tool-button--ghost" @click="downloadQr('svg')"><Icon name="download" size="sm" aria-hidden="true" />{{ label('tools.workspace.downloadSvg', 'Download SVG') }}</button>
        </div>
        <ul v-if="statusResults.length" class="tool-results"><li v-for="status in statusResults" :key="status.code"><strong>{{ status.code }} {{ status.name }}</strong><span>{{ status.meaning }}</span></li></ul>
        <ul v-if="diffResults.length" class="tool-results tool-results--diff"><li v-for="(line, index) in diffResults" :key="`${line.type}-${index}`" :class="`is-${line.type}`"><span>{{ line.type === 'added' ? '+' : line.type === 'removed' ? '-' : ' ' }}</span>{{ line.value }}</li></ul>
        <ul v-if="characterDiffResults.length" class="tool-results tool-results--diff"><li v-for="(part, index) in characterDiffResults" :key="`${part.type}-${index}`" :class="`is-${part.type}`"><span>{{ part.type === 'added' ? '+' : part.type === 'removed' ? '-' : ' ' }}</span>{{ part.value }}</li></ul>
        <div v-if="!outputText && !outputHtml && !liveMarkdownHtml && !qrDataUrl && !statusResults.length && !diffResults.length && !characterDiffResults.length && !error" class="tool-empty">
          <span class="tool-empty__icon" aria-hidden="true"><Icon :name="definition.icon" size="lg" /></span>
          <strong>{{ label('tools.workspace.emptyTitle', 'No result yet') }}</strong>
          <span>{{ label('tools.workspace.emptyDescription', 'Complete the input and run the tool to see the result here.') }}</span>
        </div>
      </section>

      <section class="tool-panel tool-guide" aria-labelledby="tool-guide-title">
        <div class="tool-guide__header">
          <span class="tool-guide__icon" aria-hidden="true"><Icon name="lightbulb" size="md" /></span>
          <div>
            <h2 id="tool-guide-title">{{ label('tools.workspace.guideTitle', 'How to use') }}</h2>
            <p>{{ label('tools.workspace.guideSubtitle', 'A short, repeatable workflow for this utility.') }}</p>
          </div>
        </div>
        <ol class="tool-guide__steps">
          <li>
            <span>1</span>
            <div><strong>{{ label('tools.workspace.guideInputTitle', 'Enter parameters') }}</strong><p>{{ label('tools.workspace.guideInputDescription', 'Provide the values required by the tool on the left.') }}</p></div>
          </li>
          <li>
            <span>2</span>
            <div><strong>{{ label('tools.workspace.guideRunTitle', 'Run locally') }}</strong><p>{{ label('tools.workspace.guideRunDescription', 'Start the operation without sending the input to ModuRelay.') }}</p></div>
          </li>
          <li>
            <span>3</span>
            <div><strong>{{ label('tools.workspace.guideOutputTitle', 'Use the result') }}</strong><p>{{ label('tools.workspace.guideOutputDescription', 'Review, copy, or download the generated result when available.') }}</p></div>
          </li>
        </ol>
        <p class="tool-guide__privacy"><Icon name="lock" size="sm" aria-hidden="true" />{{ label('tools.workspace.privacyBanner', 'Tool inputs are processed locally in your browser.') }}</p>
      </section>
    </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import Icon from '@/components/icons/Icon.vue'
import { getDefaultToolOperation } from './defaults'
import { getToolDefinition, type ToolId } from './registry'
import {
  calculateCidr, currentTimestamp, curlToSnippets, dateToTimestamp, decodeBase64Utf8, decodeJwt, decodeUrlComponent, diffCharacters, diffLines, encodeBase64Utf8, encodeUrlComponent, estimateEntropyBits, executeRegexInWorker, formatJson, generateApiSnippets, generatePassword, generateRandomString, generateTotp, generateUuidV4, generateUuidV7, getRandomAlphabetSize, hashText, hmacText, jsonToGo, jsonToPython, jsonToTypeScript, markdownToSafeHtml, minifyJson, nextCronRuns, parseCron, parseOtpAuthUri, parseUserAgent, searchHttpStatuses, timestampToDate, validateApiRequestUrl, validateJson, validateQrInput, type ApiRequestSpec, type CharacterDiff, type DiffLine, type HashAlgorithm, type RandomStringOptions,
} from './core'

const props = defineProps<{ tool: ToolId }>()
const { t } = useI18n()
const appStore = useAppStore()
const definition = computed(() => getToolDefinition(props.tool) || { id: props.tool, route: `/tools/${props.tool}`, titleKey: props.tool, descriptionKey: '', icon: 'cube' as const, category: 'developer' as const, enabled: true as const })
const noticeKey = computed(() => {
  if (props.tool === 'jwt') return 'tools.notices.jwt'
  if (props.tool === 'api-builder') return 'tools.notices.request'
  if (props.tool === 'curl-converter') return 'tools.notices.curl'
  if (props.tool === 'regex') return 'tools.notices.regex'
  return ''
})
const categoryLabel = computed(() => label('tools.categories.' + definition.value.category, definition.value.category))
const actionLabel = computed(() => label('tools.workspace.actions.' + props.tool, label('tools.workspace.run', 'Run locally')))
const input = ref('')
const secret = ref('')
const showSecret = ref(false)
const showAuthValue = ref(false)
const showJwt = ref(false)
const pattern = ref('')
const flags = ref('g')
const operation = ref(getDefaultToolOperation(props.tool))
const timestampUnit = ref<'auto' | 'seconds' | 'milliseconds'>('auto')
const algorithm = ref<HashAlgorithm>('SHA-256')
const outputText = ref('')
const outputHtml = ref('')
const liveMarkdownHtml = ref('')
const qrDataUrl = ref('')
const qrSvg = ref('')
const error = ref('')
const copied = ref(false)
const isRunning = ref(false)
const batch = ref(1)
const indent = ref(2)
const urlSafe = ref(false)
const uuidVariant = ref<'v4' | 'v7'>('v4')
const rootName = ref('Root')
const language = ref<'typescript' | 'go' | 'python'>('typescript')
const original = ref('')
const modified = ref('')
const diffMode = ref<'lines' | 'characters'>('lines')
const ignoreCase = ref(false)
const ignoreWhitespace = ref(false)
const qrSize = ref(256)
const qrLevel = ref<'L' | 'M' | 'Q' | 'H'>('M')
const totpDetails = ref<{ code: string; remainingSeconds: number; issuer?: string; account?: string; period: number; digits: number; algorithm: string } | null>(null)
let totpInterval: number | undefined
let markdownPreviewTimer: number | undefined
const statusResults = ref<ReturnType<typeof searchHttpStatuses>>([])
const diffResults = ref<DiffLine[]>([])
const characterDiffResults = ref<CharacterDiff[]>([])

const randomAlphabetSize = computed(() => getRandomAlphabetSize(options))
const randomEntropyBits = computed(() => estimateEntropyBits(options.length, randomAlphabetSize.value))
const randomStrength = computed(() => {
  if (randomEntropyBits.value >= 128) return label('tools.workspace.strengthVeryStrong', 'Very strong')
  if (randomEntropyBits.value >= 80) return label('tools.workspace.strengthStrong', 'Strong')
  if (randomEntropyBits.value >= 50) return label('tools.workspace.strengthModerate', 'Moderate')
  return label('tools.workspace.strengthWeak', 'Weak')
})
const options = reactive({ length: 24, uppercase: true, lowercase: true, numbers: true, symbols: true, customCharacters: '', excludeCharacters: '' }) satisfies RandomStringOptions
type ApiAuthType = NonNullable<ApiRequestSpec['authorization']>['type']
type ApiBuilderState = {
  url: string
  method: ApiRequestSpec['method']
  query: string
  headers: string
  body: string
  bodyType: NonNullable<ApiRequestSpec['bodyType']>
  authType: 'none' | ApiAuthType
  authValue: string
  authUsername: string
  authHeaderName: string
}
const api = reactive<ApiBuilderState>({ url: '', method: 'GET', query: '{}', headers: '{}', body: '', bodyType: 'json', authType: 'none', authValue: '', authUsername: '', authHeaderName: 'X-API-Key' })
const methods: readonly ApiRequestSpec['method'][] = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']

function label(key: string, fallback: string): string {
  const translated = t(key)
  return translated === key ? fallback : translated
}

function resetOutput(): void {
  outputText.value = ''
  outputHtml.value = ''
  liveMarkdownHtml.value = ''
  qrDataUrl.value = ''
  qrSvg.value = ''
  error.value = ''
  copied.value = false
  statusResults.value = []
  diffResults.value = []
  characterDiffResults.value = []
  totpDetails.value = null
}

function stopTotpRefresh(): void {
  if (totpInterval !== undefined) {
    window.clearInterval(totpInterval)
    totpInterval = undefined
  }
}

async function refreshTotp(config: Parameters<typeof generateTotp>[0]): Promise<void> {
  const result = await generateTotp(config)
  totpDetails.value = { ...result, issuer: config.issuer, account: config.account }
  outputText.value = result.code
}

function startTotpRefresh(config: Parameters<typeof generateTotp>[0]): void {
  stopTotpRefresh()
  totpInterval = window.setInterval(() => {
    void refreshTotp(config).catch((cause) => {
      error.value = cause instanceof Error ? cause.message : label('tools.workspace.failed', 'Unable to process this input')
      stopTotpRefresh()
    })
  }, 1000)
}

function resetTool(): void {
  stopTotpRefresh()
  input.value = ''
  secret.value = ''
  showSecret.value = false
  showAuthValue.value = false
  showJwt.value = false
  pattern.value = ''
  flags.value = 'g'
  operation.value = getDefaultToolOperation(props.tool)
  timestampUnit.value = 'auto'
  algorithm.value = 'SHA-256'
  batch.value = 1
  indent.value = 2
  urlSafe.value = false
  uuidVariant.value = 'v4'
  rootName.value = 'Root'
  language.value = 'typescript'
  original.value = ''
  modified.value = ''
  diffMode.value = 'lines'
  ignoreCase.value = false
  ignoreWhitespace.value = false
  qrSize.value = 256
  qrLevel.value = 'M'
  options.length = 24
  options.uppercase = true
  options.lowercase = true
  options.numbers = true
  options.symbols = true
  options.customCharacters = ''
  options.excludeCharacters = ''
  api.url = ''
  api.method = 'GET'
  api.query = '{}'
  api.headers = '{}'
  api.body = ''
  api.bodyType = 'json'
  api.authType = 'none'
  api.authValue = ''
  api.authUsername = ''
  api.authHeaderName = 'X-API-Key'
  resetOutput()
}

function scheduleMarkdownPreview(): void {
  if (markdownPreviewTimer !== undefined) window.clearTimeout(markdownPreviewTimer)
  if (props.tool !== 'markdown' || !input.value.trim()) {
    liveMarkdownHtml.value = ''
    if (props.tool === 'markdown') outputHtml.value = ''
    return
  }
  outputHtml.value = ''
  markdownPreviewTimer = window.setTimeout(() => {
    try {
      liveMarkdownHtml.value = markdownToSafeHtml(input.value)
    } catch {
      liveMarkdownHtml.value = ''
    }
    markdownPreviewTimer = undefined
  }, 120)
}

function showCurrentTimestamp(): void {
  resetOutput()
  const result = currentTimestamp()
  input.value = String(result.milliseconds)
  timestampUnit.value = 'milliseconds'
  outputText.value = JSON.stringify(result, null, 2)
}

function jsonStringMap(value: string, field: string): Record<string, string> {
  if (!value.trim()) return {}
  const parsed: unknown = JSON.parse(value)
  if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) throw new Error(`${field} must be a JSON object`)
  return Object.fromEntries(Object.entries(parsed).map(([key, item]) => [key, String(item)]))
}

function requireInteger(value: number, min: number, max: number): number {
  if (!Number.isInteger(value) || value < min || value > max) {
    throw new Error(`${label('tools.workspace.invalidNumber', 'Enter a whole number in range')} (${min}-${max})`)
  }
  return value
}

function validateApiBuilderUrl(value: string): string {
  try {
    return validateApiRequestUrl(value)
  } catch (cause) {
    const message = cause instanceof Error ? cause.message : ''
    if (message === 'API URL is required') {
      throw new Error(label('tools.workspace.apiUrlRequired', 'API URL is required'))
    }
    throw new Error(label('tools.workspace.apiUrlInvalid', 'Enter an absolute HTTP(S) URL'))
  }
}

function requestAuthorization(): ApiRequestSpec['authorization'] {
  if (api.authType === 'none') return undefined
  if (!api.authValue.trim()) throw new Error(label('tools.workspace.authValueRequired', 'Authorization value is required'))
  if (api.authType === 'basic' && !api.authUsername.trim()) throw new Error(label('tools.workspace.authUsernameRequired', 'Basic authentication username is required'))
  if (api.authType === 'api-key' && !api.authHeaderName.trim()) throw new Error(label('tools.workspace.authHeaderRequired', 'API key header name is required'))
  if (api.authType === 'basic') return { type: 'basic', value: api.authValue, username: api.authUsername }
  if (api.authType === 'api-key') return { type: 'api-key', value: api.authValue, headerName: api.authHeaderName }
  return { type: 'bearer', value: api.authValue }
}

async function runTool(): Promise<void> {
  if (isRunning.value) return
  resetOutput()
  isRunning.value = true
  try {
    switch (props.tool) {
      case 'totp': {
        const config = input.value.toLowerCase().startsWith('otpauth://') ? parseOtpAuthUri(input.value) : { secret: input.value, algorithm: 'SHA-1' as const, digits: 6 as const, period: 30 }
        const parsedConfig = input.value.toLowerCase().startsWith('otpauth://') ? config : { ...config, issuer: undefined, account: undefined }
        await refreshTotp(parsedConfig)
        startTotpRefresh(parsedConfig)
        break
      }
      case 'password':
        requireInteger(options.length, 1, 8192)
        requireInteger(batch.value, 1, 20)
        if (options.length * batch.value > 200_000) throw new Error(label('tools.workspace.outputTooLarge', 'Reduce length or batch size before generating the result'))
        outputText.value = Array.from({ length: batch.value }, () => generatePassword(options)).join('\n')
        break
      case 'random-string':
        requireInteger(options.length, 1, 8192)
        requireInteger(batch.value, 1, 20)
        if (options.length * batch.value > 200_000) throw new Error(label('tools.workspace.outputTooLarge', 'Reduce length or batch size before generating the result'))
        outputText.value = Array.from({ length: batch.value }, () => generateRandomString(options)).join('\n')
        break
      case 'uuid':
        requireInteger(batch.value, 1, 100)
        outputText.value = Array.from({ length: batch.value }, () => uuidVariant.value === 'v7' ? generateUuidV7() : generateUuidV4()).join('\n')
        break
      case 'json':
        if (operation.value === 'format') requireInteger(indent.value, 0, 10)
        outputText.value = operation.value === 'format' ? formatJson(input.value, indent.value) : operation.value === 'minify' ? minifyJson(input.value) : JSON.stringify(validateJson(input.value), null, 2)
        break
      case 'base64':
        outputText.value = operation.value === 'encode' ? encodeBase64Utf8(input.value, urlSafe.value) : decodeBase64Utf8(input.value, urlSafe.value)
        break
      case 'url':
        outputText.value = operation.value === 'encode' ? encodeUrlComponent(input.value) : decodeUrlComponent(input.value)
        break
      case 'timestamp':
        outputText.value = operation.value === 'timestampToDate' ? JSON.stringify(timestampToDate(input.value, timestampUnit.value), null, 2) : JSON.stringify(dateToTimestamp(input.value), null, 2)
        break
      case 'jwt':
        outputText.value = JSON.stringify(decodeJwt(input.value), null, 2)
        break
      case 'hash':
        outputText.value = JSON.stringify(await hashText(input.value, algorithm.value), null, 2)
        break
      case 'hmac':
        outputText.value = JSON.stringify(await hmacText(input.value, secret.value, algorithm.value as Exclude<HashAlgorithm, 'SHA-1'>), null, 2)
        break
      case 'regex':
        outputText.value = JSON.stringify(await executeRegexInWorker(pattern.value, flags.value, input.value), null, 2)
        break
      case 'http-status':
        statusResults.value = searchHttpStatuses(input.value)
        break
      case 'user-agent':
        outputText.value = JSON.stringify(parseUserAgent(input.value), null, 2)
        break
      case 'ip-cidr':
        outputText.value = JSON.stringify(calculateCidr(input.value), null, 2)
        break
      case 'api-builder':
        outputText.value = JSON.stringify(generateApiSnippets({ method: api.method, url: validateApiBuilderUrl(api.url), query: jsonStringMap(api.query, label('tools.workspace.query', 'Query parameters')), headers: jsonStringMap(api.headers, label('tools.workspace.headers', 'Headers')), body: api.body || undefined, bodyType: api.body ? api.bodyType : undefined, authorization: requestAuthorization() }), null, 2)
        break
      case 'curl-converter':
        outputText.value = JSON.stringify(curlToSnippets(input.value), null, 2)
        break
      case 'json-to-struct':
        outputText.value = language.value === 'typescript' ? jsonToTypeScript(input.value, { rootName: rootName.value }) : language.value === 'go' ? jsonToGo(input.value, { rootName: rootName.value }) : jsonToPython(input.value, { rootName: rootName.value })
        break
      case 'cron':
        requireInteger(batch.value, 1, 20)
        outputText.value = JSON.stringify({ ...parseCron(input.value), timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'local', nextRuns: nextCronRuns(input.value, batch.value).map((date) => date.toISOString()) }, null, 2)
        break
      case 'diff':
        if (diffMode.value === 'characters') {
          characterDiffResults.value = diffCharacters(original.value, modified.value)
        } else {
          diffResults.value = diffLines(original.value, modified.value, { ignoreCase: ignoreCase.value, ignoreWhitespace: ignoreWhitespace.value })
        }
        break
      case 'markdown':
        outputHtml.value = markdownToSafeHtml(input.value)
        break
      case 'qrcode': {
        const qr = validateQrInput(input.value, { size: qrSize.value, errorCorrection: qrLevel.value })
        const module = await import('qrcode')
        qrDataUrl.value = await module.toDataURL(qr.value, { width: qr.size, errorCorrectionLevel: qr.errorCorrection })
        qrSvg.value = await module.toString(qr.value, { type: 'svg', width: qr.size, errorCorrectionLevel: qr.errorCorrection })
        break
      }
    }
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : label('tools.workspace.failed', 'Unable to process this input')
  } finally {
    isRunning.value = false
  }
}

async function copyOutput(): Promise<void> {
  const value = outputText.value || liveMarkdownHtml.value || outputHtml.value
  if (!value) return
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(value)
    } else {
      const textarea = document.createElement('textarea')
      try {
        textarea.value = value
        textarea.setAttribute('readonly', '')
        textarea.style.position = 'fixed'
        textarea.style.opacity = '0'
        document.body.appendChild(textarea)
        textarea.select()
        if (!document.execCommand('copy')) throw new Error('Clipboard is unavailable')
      } finally {
        textarea.remove()
      }
    }
    copied.value = true
    appStore.showSuccess(label('tools.workspace.copied', 'Copied'))
    window.setTimeout(() => { copied.value = false }, 1500)
  } catch {
    error.value = label('tools.workspace.copyFailed', 'Copy failed; copy the output manually')
  }
}

function triggerDownload(content: string, filename: string, type: string): void {
  const url = URL.createObjectURL(new Blob([content], { type }))
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  anchor.click()
  window.setTimeout(() => URL.revokeObjectURL(url), 0)
}

function downloadOutput(): void {
  const value = outputText.value || liveMarkdownHtml.value || outputHtml.value
  if (!value) return
  const isHtml = props.tool === 'markdown' && Boolean(liveMarkdownHtml.value || outputHtml.value)
  const extension = isHtml ? 'html' : props.tool === 'json' ? 'json' : 'txt'
  triggerDownload(value, `${props.tool}-output.${extension}`, isHtml ? 'text/html;charset=utf-8' : 'text/plain;charset=utf-8')
}

function downloadQr(format: 'png' | 'svg'): void {
  if (format === 'png' && qrDataUrl.value) {
    const anchor = document.createElement('a')
    anchor.href = qrDataUrl.value
    anchor.download = `${props.tool}.png`
    anchor.click()
  } else if (format === 'svg' && qrSvg.value) {
    triggerDownload(qrSvg.value, `${props.tool}.svg`, 'image/svg+xml;charset=utf-8')
  }
}

watch(() => props.tool, () => { resetTool(); scheduleMarkdownPreview() })
watch(input, scheduleMarkdownPreview)
onBeforeUnmount(() => {
  stopTotpRefresh()
  if (markdownPreviewTimer !== undefined) window.clearTimeout(markdownPreviewTimer)
})
</script>

<style scoped>
.tool-workspace {
  width: 100%;
  color: var(--color-text-primary);
}

.tool-workspace__header {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(340px, .8fr);
  gap: 18px;
  margin-bottom: 18px;
}

.tool-workspace__heading {
  min-width: 0;
  padding: 10px 0 6px;
}

.tool-workspace__eyebrow {
  display: block;
  margin-bottom: 8px;
  color: var(--color-accent);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: .72rem;
  font-weight: 750;
  letter-spacing: .12em;
  text-transform: uppercase;
}

.tool-workspace h1 {
  margin: 0;
  font-size: clamp(1.75rem, 3vw, 2.2rem);
  font-weight: 700;
  line-height: 1.18;
  letter-spacing: -.035em;
}

.tool-workspace__heading > p {
  max-width: 690px;
  margin: 9px 0 0;
  color: var(--color-text-secondary);
  font-size: .88rem;
  line-height: 1.62;
}

.tool-workspace__badges {
  display: flex;
  flex-wrap: wrap;
  gap: 7px;
  margin-top: 14px;
}

.tool-workspace__badges span {
  display: inline-flex;
  min-height: 32px;
  align-items: center;
  gap: 6px;
  padding: 0 10px;
  border: 1px solid var(--color-border);
  border-radius: 999px;
  background: var(--color-surface);
  color: var(--color-text-secondary);
  font-size: .72rem;
  font-weight: 600;
}

.tool-workspace__badges span:nth-child(2) svg {
  color: var(--color-accent);
}

.tool-workspace__badges span:nth-child(1) svg,
.tool-workspace__badges span:nth-child(3) svg {
  color: var(--color-primary);
}

.tool-workspace__context {
  display: grid;
  min-width: 0;
  grid-template-columns: 120px minmax(0, 1fr);
  align-items: center;
  gap: 16px;
  padding: 18px;
  border: 1px solid var(--glass-border);
  border-radius: 16px;
  background: var(--color-surface);
  background: var(--glass-bg);
  box-shadow: var(--glass-shadow);
  -webkit-backdrop-filter: blur(16px);
  backdrop-filter: blur(16px);
}

.tool-workspace__context-art {
  position: relative;
  min-height: 116px;
}

.tool-workspace__context-card {
  position: absolute;
  display: grid;
  width: 78px;
  height: 88px;
  place-items: center;
  border: 1px solid var(--color-primary-border);
  border-radius: 16px;
  background: var(--color-surface-raised);
  color: var(--color-primary);
  box-shadow: var(--shadow-sm);
}

.tool-workspace__context-card--back {
  top: 6px;
  left: 3px;
  opacity: .5;
}

.tool-workspace__context-card--front {
  top: 20px;
  left: 34px;
}

.tool-workspace__context-copy {
  min-width: 0;
}

.tool-workspace__context-copy > strong {
  display: block;
  font-size: .9rem;
  font-weight: 700;
}

.tool-workspace__context-copy > p {
  margin: 5px 0 0;
  color: var(--color-text-muted);
  font-size: .74rem;
  line-height: 1.5;
}

.tool-workspace__context-copy ul {
  display: grid;
  gap: 5px;
  margin: 10px 0 0;
  padding: 0;
  list-style: none;
}

.tool-workspace__context-copy li {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--color-text-secondary);
  font-size: .7rem;
}

.tool-workspace__context-copy li svg {
  color: var(--color-primary);
}

.tool-workspace__notice {
  margin: 0 0 14px;
  padding: 10px 12px;
  border: 1px solid var(--color-border);
  border-radius: 10px;
  background: var(--color-surface-soft);
  color: var(--color-text-secondary);
  font-size: .8rem;
  line-height: 1.5;
}

.tool-workspace__grid {
  display: grid;
  grid-template-columns: minmax(360px, .94fr) minmax(0, 1.06fr);
  gap: 14px;
  align-items: start;
}

.tool-workspace__side {
  display: grid;
  gap: 14px;
  min-width: 0;
}

.tool-panel {
  min-width: 0;
  padding: 18px;
  border: 1px solid var(--color-border);
  border-radius: 16px;
  background: var(--color-surface);
  box-shadow: var(--shadow-xs);
}

.tool-panel--input,
.tool-panel--output {
  min-height: 420px;
}

.tool-panel--output {
  display: flex;
  flex-direction: column;
}

.tool-panel__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 18px;
  padding-bottom: 13px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.tool-panel__title {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  gap: 10px;
}

.tool-panel__step {
  display: grid;
  width: 30px;
  height: 30px;
  flex: 0 0 30px;
  place-items: center;
  border-radius: 50%;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: .74rem;
  font-weight: 800;
}

.tool-panel h2,
.tool-guide h2 {
  margin: 0;
  font-size: .95rem;
  font-weight: 680;
}

.tool-panel__title p,
.tool-guide__header p {
  margin: 3px 0 0;
  color: var(--color-text-muted);
  font-size: .7rem;
  line-height: 1.45;
}

.tool-panel__actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 7px;
}

.tool-panel__actions--qr {
  justify-content: center;
  margin-top: 8px;
}

.tool-field {
  display: grid;
  gap: 7px;
  margin-bottom: 14px;
  color: var(--color-text-secondary);
  font-size: .8rem;
  font-weight: 600;
}

.tool-input {
  width: 100%;
  min-height: 42px;
  box-sizing: border-box;
  padding: 9px 11px;
  border: 1px solid var(--color-border-strong);
  border-radius: 10px;
  outline: none;
  background: var(--color-bg-subtle);
  color: var(--color-text-primary);
  font: inherit;
  resize: vertical;
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    box-shadow var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard);
}

textarea.tool-input {
  line-height: 1.55;
}

.tool-input:hover:not(:focus) {
  border-color: var(--color-primary-border);
}

.tool-input:focus {
  border-color: var(--color-primary);
  background: var(--color-surface-raised);
  box-shadow: 0 0 0 3px var(--color-primary-ring);
}

.tool-input-wrap {
  position: relative;
  display: block;
}

.tool-input--with-action {
  padding-right: 42px;
}

.tool-input-action {
  position: absolute;
  top: 50%;
  right: 2px;
  display: inline-flex;
  width: 38px;
  height: 38px;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--color-text-muted);
  cursor: pointer;
  transform: translateY(-50%);
}

.tool-input-action:hover {
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.tool-input-action:focus-visible,
.tool-button:focus-visible {
  outline: 2px solid var(--color-primary-ring);
  outline-offset: 2px;
}

.tool-input--mono,
.tool-output {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

.tool-checks {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin-bottom: 13px;
  color: var(--color-text-secondary);
  font-size: .78rem;
}

.tool-check,
.tool-checks label {
  display: flex;
  min-height: 34px;
  align-items: center;
  gap: 7px;
}

.tool-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 10px;
}

.tool-help {
  color: var(--color-text-muted);
  font-size: .75rem;
  line-height: 1.5;
}

.tool-button {
  display: inline-flex;
  min-height: 36px;
  align-items: center;
  justify-content: center;
  gap: 7px;
  padding: 7px 13px;
  border: 1px solid transparent;
  border-radius: 9px;
  cursor: pointer;
  font: inherit;
  font-size: .78rem;
  font-weight: 650;
  transition:
    background-color var(--motion-fast) var(--ease-standard),
    border-color var(--motion-fast) var(--ease-standard),
    color var(--motion-fast) var(--ease-standard),
    box-shadow var(--motion-fast) var(--ease-standard),
    transform var(--motion-fast) var(--ease-standard);
}

.tool-button--primary {
  width: 100%;
  min-height: 42px;
  margin-top: 5px;
  border-color: var(--color-primary);
  background: var(--color-primary);
  color: var(--color-surface);
  box-shadow: var(--shadow-xs);
}

.tool-button--primary:hover:not(:disabled) {
  background: var(--color-primary-hover);
  transform: translateY(-1px);
}

.tool-button--primary:disabled {
  cursor: wait;
  opacity: .72;
}

.tool-button--ghost {
  border-color: var(--color-border);
  background: var(--color-surface);
  color: var(--color-text-secondary);
}

.tool-button--ghost:hover {
  border-color: var(--color-primary-border);
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.tool-spinner {
  width: 13px;
  height: 13px;
  border: 2px solid currentColor;
  border-right-color: transparent;
  border-radius: 50%;
  animation: tool-spin .7s linear infinite;
}

.tool-output {
  max-height: 600px;
  min-height: 210px;
  margin: 0;
  padding: 14px;
  overflow: auto;
  border: 1px solid var(--color-border);
  border-radius: 10px;
  background: var(--color-bg-subtle);
  color: var(--color-text-primary);
  font-size: .76rem;
  line-height: 1.6;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.tool-totp-result {
  display: grid;
  gap: 7px;
  margin-bottom: 12px;
  padding: 18px;
  border: 1px solid var(--color-primary-border);
  border-radius: 12px;
  background: var(--color-primary-soft);
  color: var(--color-text-secondary);
  font-size: .8rem;
}

.tool-totp-result__code {
  color: var(--color-primary-active);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 2.25rem;
  font-weight: 750;
  letter-spacing: .1em;
  line-height: 1.1;
}

.tool-markdown {
  min-height: 220px;
  padding: 14px;
  overflow: auto;
  border: 1px solid var(--color-border);
  border-radius: 10px;
  background: var(--color-surface);
  line-height: 1.6;
}

.tool-error {
  margin: 0 0 12px;
  padding: 10px 12px;
  border: 1px solid color-mix(in srgb, var(--color-danger) 35%, transparent);
  border-radius: 10px;
  background: color-mix(in srgb, var(--color-danger) 8%, transparent);
  color: var(--color-danger);
  font-size: .8rem;
  line-height: 1.45;
}

.tool-results {
  display: grid;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.tool-results li {
  display: grid;
  gap: 3px;
  padding: 10px 12px;
  border: 1px solid var(--color-border);
  border-radius: 9px;
  background: var(--color-surface-soft);
  font-size: .8rem;
}

.tool-results li span {
  color: var(--color-text-secondary);
}

.tool-results--diff li {
  display: flex;
  gap: 8px;
  border-radius: 0;
  border-width: 0 0 1px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  white-space: pre-wrap;
}

.tool-results--diff .is-added {
  color: var(--color-success);
  background: color-mix(in srgb, var(--color-success) 8%, transparent);
}

.tool-results--diff .is-removed {
  color: var(--color-danger);
  background: color-mix(in srgb, var(--color-danger) 8%, transparent);
}

.tool-qr {
  display: block;
  width: min(100%, 360px);
  height: auto;
  margin: 12px auto;
  image-rendering: pixelated;
}

.tool-empty {
  display: grid;
  flex: 1;
  min-height: 285px;
  place-items: center;
  align-content: center;
  gap: 8px;
  padding: 24px;
  border: 1px solid var(--color-border-subtle);
  border-radius: 12px;
  background: var(--color-surface-soft);
  text-align: center;
}

.tool-empty__icon {
  display: grid;
  width: 54px;
  height: 54px;
  place-items: center;
  margin-bottom: 6px;
  border: 1px solid var(--color-primary-border);
  border-radius: 15px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.tool-empty strong {
  color: var(--color-text-primary);
  font-size: .9rem;
  font-weight: 680;
}

.tool-empty > span:last-child {
  max-width: 360px;
  color: var(--color-text-muted);
  font-size: .76rem;
  line-height: 1.55;
}

.tool-guide {
  min-height: auto;
}

.tool-guide__header {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding-bottom: 13px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.tool-guide__icon {
  display: grid;
  width: 34px;
  height: 34px;
  flex: 0 0 34px;
  place-items: center;
  border-radius: 10px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.tool-guide__steps {
  display: grid;
  gap: 0;
  margin: 14px 0 0;
  padding: 0;
  list-style: none;
}

.tool-guide__steps li {
  display: grid;
  min-height: 54px;
  grid-template-columns: 28px minmax(0, 1fr);
  gap: 10px;
  align-items: start;
  padding: 8px 0;
}

.tool-guide__steps li > span {
  display: grid;
  width: 28px;
  height: 28px;
  place-items: center;
  border-radius: 50%;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: .7rem;
  font-weight: 800;
}

.tool-guide__steps strong {
  display: block;
  color: var(--color-text-primary);
  font-size: .78rem;
  font-weight: 650;
}

.tool-guide__steps p {
  margin: 3px 0 0;
  color: var(--color-text-muted);
  font-size: .72rem;
  line-height: 1.5;
}

.tool-guide__privacy {
  display: flex;
  align-items: flex-start;
  gap: 7px;
  margin: 12px 0 0;
  padding: 10px 11px;
  border: 1px solid color-mix(in srgb, var(--color-success) 24%, var(--color-border));
  border-radius: 10px;
  background: color-mix(in srgb, var(--color-success) 7%, var(--color-surface));
  color: var(--color-text-secondary);
  font-size: .72rem;
  line-height: 1.5;
}

.tool-guide__privacy svg {
  flex: 0 0 auto;
  color: var(--color-success);
}

@keyframes tool-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 980px) {
  .tool-workspace__header,
  .tool-workspace__grid {
    grid-template-columns: 1fr;
  }

  .tool-workspace__context {
    grid-template-columns: 105px minmax(0, 1fr);
  }

  .tool-panel--input,
  .tool-panel--output {
    min-height: auto;
  }
}

@media (max-width: 560px) {
  .tool-workspace__context {
    display: none;
  }

  .tool-workspace h1 {
    font-size: 1.65rem;
  }

  .tool-panel {
    padding: 14px;
    border-radius: 13px;
  }

  .tool-panel__header {
    align-items: flex-start;
  }

  .tool-checks,
  .tool-row {
    grid-template-columns: 1fr;
  }

  .tool-button {
    min-height: 40px;
  }

  .tool-empty {
    min-height: 220px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .tool-input,
  .tool-button {
    transition-duration: .01ms;
  }

  .tool-button--primary:hover:not(:disabled) {
    transform: none;
  }

  .tool-spinner {
    animation: none;
  }
}
</style>
