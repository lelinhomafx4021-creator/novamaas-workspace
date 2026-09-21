import { describe, expect, it } from 'vitest'

import { buildQuery } from '../query'

describe('buildQuery', () => {
  it('encodes values and omits empty optional parameters', () => {
    expect(buildQuery({ model_name: '模型 / pro', page: 1, request_id: '' })).toBe(
      'model_name=%E6%A8%A1%E5%9E%8B%20%2F%20pro&page=1'
    )
  })
})
