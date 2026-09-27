export type PreviewMediaKind = 'image' | 'video'

export function buildMediaPreviewRoute(kind: PreviewMediaKind, filePath: string) {
  return `/pages/media-preview/index?kind=${kind}&src=${encodeURIComponent(filePath)}`
}

export function readMediaPreviewRoute(params: Record<string, string | undefined>) {
  if ((params.kind !== 'image' && params.kind !== 'video') || !params.src) {
    return null
  }
  return { kind: params.kind, filePath: params.src }
}
