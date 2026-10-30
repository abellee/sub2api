export interface CreativeDocument {
  id: string
  label: string
  kind: 'image' | 'video'
}
export const documentCatalog: CreativeDocument[]
