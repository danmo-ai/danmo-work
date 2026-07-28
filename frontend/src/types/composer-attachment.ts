import {
  buildUserInputWithAttachments,
  type ElementAttachment,
} from '@/types/element-attachment'
import {
  serializeCodeSelectionAttachments,
  type CodeSelectionAttachment,
} from '@/types/code-attachment'

export type ComposerAttachmentKind = 'image' | 'file' | 'element' | 'code'

export interface ImageComposerAttachment {
  id: string
  kind: 'image'
  name: string
  mime: string
  size: number
  dataUrl: string
}

export interface FileComposerAttachment {
  id: string
  kind: 'file'
  name: string
  mime: string
  size: number
  /** Placeholder until file upload pipeline exists. */
  placeholder: true
}

export interface ElementComposerAttachment {
  id: string
  kind: 'element'
  data: ElementAttachment
}

export interface CodeComposerAttachment {
  id: string
  kind: 'code'
  data: CodeSelectionAttachment
}

export type ComposerAttachment =
  | ImageComposerAttachment
  | FileComposerAttachment
  | ElementComposerAttachment
  | CodeComposerAttachment

/** API payload for vision models (matches domain.UserAttachment). */
export interface ApiUserAttachment {
  type: 'image'
  name?: string
  mimeType?: string
  data: string // raw base64 or data URL
}

export const MAX_IMAGE_ATTACHMENT_BYTES = 10 * 1024 * 1024

export function createComposerAttachmentId(prefix: string): string {
  return `${prefix}_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`
}

export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / (1024 * 1024)).toFixed(1)} MB`
}

/** Text + element/code/file placeholders; images are sent as structured attachments. */
export function buildComposerUserInput(
  text: string,
  atts: ComposerAttachment[],
): string {
  const elements = atts
    .filter((a): a is ElementComposerAttachment => a.kind === 'element')
    .map((a) => a.data)
  const codes = atts
    .filter((a): a is CodeComposerAttachment => a.kind === 'code')
    .map((a) => a.data)
  const files = atts.filter((a): a is FileComposerAttachment => a.kind === 'file')

  let body = buildUserInputWithAttachments(text, elements)

  const codeBlocks = serializeCodeSelectionAttachments(codes)
  if (codeBlocks) {
    body = body ? `${body}\n\n${codeBlocks}` : codeBlocks
  }

  if (files.length) {
    const blocks = files
      .map((f) => `[Attached file (pending upload): ${f.name} · ${f.mime} · ${formatBytes(f.size)}]`)
      .join('\n')
    body = body ? `${body}\n\n${blocks}` : blocks
  }

  return body
}

export function toApiImageAttachments(atts: ComposerAttachment[]): ApiUserAttachment[] {
  const images: ApiUserAttachment[] = []

  for (const a of atts) {
    if (a.kind === 'image' && a.dataUrl) {
      images.push({
        type: 'image',
        name: a.name,
        mimeType: a.mime || 'image/png',
        data: a.dataUrl,
      })
      continue
    }
    if (a.kind === 'element' && a.data.screenshotDataUrl) {
      const dataUrl = a.data.screenshotDataUrl
      const mime = dataUrl.startsWith('data:image/png')
        ? 'image/png'
        : dataUrl.startsWith('data:image/webp')
          ? 'image/webp'
          : 'image/jpeg'
      images.push({
        type: 'image',
        name: a.data.screenshotName || `ui-element-${a.data.tag || 'el'}.jpg`,
        mimeType: mime,
        data: dataUrl,
      })
    }
  }

  return images
}
