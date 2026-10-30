export interface ReferencePage {
  id: string
  title: string
  label: string
  lead: string
  blocks: Array<
    | { type: 'heading' | 'paragraph' | 'note'; value: string }
    | { type: 'code'; lang?: string; value: string }
    | { type: 'table'; headers: string[]; rows: string[][] }
    | { type: 'list'; items: string[] }
    | { type: 'links'; items: { id: string; label: string }[] }
  >
}
export const referencePages: ReferencePage[]
