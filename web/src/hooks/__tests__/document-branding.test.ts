/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { expect, test } from 'vitest'

import { mapStatusDataToConfig } from '../use-system-config'

test('status branding keeps platform and operating entity identities separate', () => {
  const config = mapStatusDataToConfig({
    system_name: 'Example Platform',
    logo: 'https://cdn.example/platform.png',
    operating_entity_name: 'Example Operator Ltd',
    operating_entity_logo: 'https://cdn.example/operator.png',
  })

  expect(config.systemName).toBe('Example Platform')
  expect(config.logo).toBe('https://cdn.example/platform.png')
  expect(config.operatingEntityName).toBe('Example Operator Ltd')
  expect(config.operatingEntityLogo).toBe('https://cdn.example/operator.png')
})
