export type CanvasClipboardPayload =
  | { kind: 'image'; file: File }
  | { kind: 'text'; text: string }

type ClipboardReader = Pick<Clipboard, 'read' | 'readText'>

export async function readCanvasClipboard(clipboard: ClipboardReader | undefined): Promise<CanvasClipboardPayload | null> {
  if (!clipboard) return null

  if (typeof clipboard.read === 'function') {
    const items = await clipboard.read()
    for (const item of items) {
      const imageType = item.types.find(type => type.startsWith('image/'))
      if (!imageType) continue
      const blob = await item.getType(imageType)
      if (!blob.size) return null
      const extension = imageType.split('/')[1]?.replace('jpeg', 'jpg') || 'png'
      return { kind: 'image', file: new File([blob], `clipboard-image.${extension}`, { type: imageType }) }
    }
  }

  if (typeof clipboard.readText !== 'function') return null
  const text = (await clipboard.readText()).trim()
  return text ? { kind: 'text', text } : null
}
