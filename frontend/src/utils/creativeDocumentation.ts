import { documentCatalog } from '../../public/creative-docs/catalog.js'

export interface DocumentationGroup {
  id: string
  name: string
  description: string
  documentId: string
  kind: 'image' | 'video'
}

export function resolveDocumentationGroups(
  groups: { id: number; name: string; description?: string | null }[],
  assignments: Record<string, string>,
): DocumentationGroup[] {
  return groups.flatMap(group => {
    const doc = documentCatalog.find(item => item.id === assignments[String(group.id)])
    return doc ? [{ id: String(group.id), name: group.name, description: group.description ?? '', documentId: doc.id, kind: doc.kind }] : []
  })
}
