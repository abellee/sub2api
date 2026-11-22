import type { ReferencePage } from './reference-content.js'
export function documentMarkdown(page: ReferencePage): string
export function agentInstructions(page: ReferencePage, groupName: string): string
export function agentPrompt(page: ReferencePage, groupName: string, apiKey?: string): string
