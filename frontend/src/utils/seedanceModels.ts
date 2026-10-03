// Keep this fallback consistent with seedanceNonVideoModelID in upstream_models.go.
// Explicit capabilities override published model families; opaque gateway IDs
// remain selectable. Call with the resolved target so aliases keep their meaning.
const nonVideoModelID = /^(?:qwen-?\d|deepseek-(?:[vr]\d|coder|llm)|glm-\d|mistral-(?:\d|small|medium|large|nemo)|doubao-(?:pro-|lite-|embedding-|seedream-|seededit-|seed-(?:\d|code)|(?:1-5|1\.5)-))/i

export function allowsSeedanceVideoModel(modelID: string, outputModalities: readonly string[] = []): boolean {
  if (outputModalities.length > 0) {
    return outputModalities.some(output => output.trim().toLowerCase() === 'video')
  }
  return !nonVideoModelID.test(modelID.trim())
}
