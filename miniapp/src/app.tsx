import { useEffect, type PropsWithChildren } from 'react'

import { QuotaDisplayProvider } from '@/currency/context'
import { initializeI18n, syncTabBarLabels } from '@/i18n/config'

import './app.scss'

void initializeI18n()

function App(props: PropsWithChildren) {
  useEffect(() => {
    void initializeI18n().then(syncTabBarLabels)
  }, [])

  return <QuotaDisplayProvider>{props.children}</QuotaDisplayProvider>
}

export default App
