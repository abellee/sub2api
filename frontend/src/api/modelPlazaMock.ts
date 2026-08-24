import Decimal from 'decimal.js'
import type { ModelPlazaGroup, ModelPlazaResponse, PlazaModel } from './modelPlaza'

interface TokenPrice {
  input: number
  output: number
  cache?: number
}

function multiplyPrice(value: number, multiplier: number): number {
  return new Decimal(value).mul(multiplier).toNumber()
}

function tokenModel(name: string, platform: string, price: TokenPrice): PlazaModel {
  const cache = price.cache ?? multiplyPrice(price.input, 0.1)
  return {
    name,
    platform,
    pricing: {
      billing_mode: 'token',
      input_price: price.input,
      output_price: price.output,
      cache_write_price: multiplyPrice(price.input, 1.25),
      cache_read_price: cache,
      image_input_price: null,
      image_output_price: null,
      per_request_price: null,
      intervals: []
    },
    official_pricing: {
      input_price: price.input,
      output_price: price.output,
      cache_write_price: multiplyPrice(price.input, 1.25),
      cache_write_1h_price: multiplyPrice(price.input, 2),
      cache_read_price: cache
    }
  }
}

function group(
  id: number,
  name: string,
  platform: string,
  rate: number,
  models: PlazaModel[],
  options: Partial<ModelPlazaGroup> = {}
): ModelPlazaGroup {
  return {
    id,
    name,
    description: `${name} development preview group`,
    platform,
    subscription_type: 'standard',
    rate_multiplier: rate,
    peak_rate_enabled: false,
    peak_start: '',
    peak_end: '',
    peak_rate_multiplier: rate,
    is_exclusive: false,
    image_rate_independent: false,
    image_rate_multiplier: 1,
    models,
    ...options
  }
}

export function createDevModelPlazaResponse(): ModelPlazaResponse {
  const openAIModels = [
    tokenModel('gpt-5.6-sol', 'openai', { input: 2.5e-6, output: 1.5e-5 }),
    tokenModel('gpt-5.6-terra', 'openai', { input: 1.5e-6, output: 9e-6 }),
    tokenModel('gpt-5.6-luna', 'openai', { input: 8e-7, output: 4.8e-6 })
  ]
  const claudeModels = [
    tokenModel('claude-opus-4-6', 'anthropic', { input: 1.5e-5, output: 7.5e-5 }),
    tokenModel('claude-sonnet-4-6', 'anthropic', { input: 3e-6, output: 1.5e-5 }),
    tokenModel('claude-haiku-4-5', 'anthropic', { input: 1e-6, output: 5e-6 })
  ]
  const deepSeekModels = [
    tokenModel('deepseek-v3.2', 'openai', { input: 2.8e-7, output: 4.2e-7 }),
    tokenModel('deepseek-r1', 'openai', { input: 5.5e-7, output: 2.19e-6 })
  ]
  const geminiModels = [
    tokenModel('gemini-3.7-flash-high', 'gemini', { input: 5e-7, output: 3e-6 }),
    tokenModel('gemini-3.5-flash-low', 'gemini', { input: 2e-7, output: 1.2e-6 }),
    tokenModel('gemini-2.5-pro', 'gemini', { input: 1.25e-6, output: 1e-5 })
  ]
  const grokModels = [
    tokenModel('grok-4-fast', 'grok', { input: 2e-7, output: 5e-7 }),
    tokenModel('grok-4.1', 'grok', { input: 3e-6, output: 1.5e-5 })
  ]

  return {
    demo: true,
    description: '**本地演示数据**：仅用于预览模型厂商与分组的多级布局，不代表线上价格。',
    groups: [
      group(101, 'GPT 经济线路', 'openai', 0.65, openAIModels.slice(1)),
      group(102, 'GPT 标准线路', 'openai', 0.85, openAIModels),
      group(103, 'Codex 高性能', 'openai', 1.05, openAIModels.slice(0, 2), { is_exclusive: true }),
      group(151, 'DeepSeek 经济线路', 'openai', 0.38, deepSeekModels),
      group(152, 'DeepSeek 稳定线路', 'openai', 0.56, deepSeekModels),
      group(201, 'Claude 日常开发', 'anthropic', 0.72, claudeModels.slice(1)),
      group(202, 'Claude 稳定线路', 'anthropic', 0.9, claudeModels),
      group(203, 'Claude 企业线路', 'anthropic', 1.15, claudeModels.slice(0, 2), {
        subscription_type: 'subscription'
      }),
      group(301, 'Gemini Flash 特惠', 'gemini', 0.42, geminiModels.slice(0, 2)),
      group(302, 'Gemini 通用线路', 'gemini', 0.68, geminiModels),
      group(303, 'Gemini Pro 专线', 'gemini', 0.88, geminiModels.slice(2), { is_exclusive: true }),
      group(401, 'Grok 极速线路', 'grok', 0.58, grokModels.slice(0, 1)),
      group(402, 'Grok 推理线路', 'grok', 0.92, grokModels),
      group(403, 'Grok 企业专线', 'grok', 1.1, grokModels, { subscription_type: 'subscription' })
    ]
  }
}
