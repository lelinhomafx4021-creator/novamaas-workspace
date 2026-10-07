import { Button, Input, Text, View } from '@tarojs/components'
import Taro, { useDidShow } from '@tarojs/taro'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ApiRequestError } from '@/api/request'
import {
  bindMiniAppAccount,
  loginWithPassword,
  loginWithWeChatPhone,
  logoutMiniApp,
  type MiniBindingRequiredData,
} from '@/auth/client'
import { getMiniAuthSession, type MiniAuthSession } from '@/auth/session'
import { PageShell } from '@/components/page-shell'
import { SmsLogin } from '@/components/sms-login'
import { WeChatBindingPanel } from '@/components/wechat-binding'
import {
  WeChatConnectionConfirmation,
  type WeChatConfirmationInput,
} from '@/components/wechat-connection'
import { usePageTitle } from '@/hooks/use-page-title'

import './index.scss'

const authErrorKeys: Record<string, string> = {
  MINI_AUTH_NOT_CONFIGURED: 'auth.error.notConfigured',
  MINI_AUTH_DISABLED: 'auth.error.notConfigured',
  MINI_AUTH_CODE_INVALID: 'auth.error.invalidCode',
  MINI_AUTH_CODE_REQUIRED: 'auth.error.invalidCode',
  MINI_AUTH_APP_ID_MISMATCH: 'auth.error.appIdMismatch',
  MINI_AUTH_CREDENTIALS_INVALID: 'auth.error.credentials',
  MINI_AUTH_2FA_REQUIRED: 'auth.error.twoFactorRequired',
  MINI_AUTH_2FA_INVALID: 'auth.error.twoFactorInvalid',
  MINI_AUTH_FLOW_INVALID: 'auth.error.flowExpired',
  MINI_AUTH_FLOW_EXPIRED: 'auth.error.flowExpired',
  MINI_AUTH_FLOW_CONSUMED: 'auth.error.flowExpired',
  MINI_AUTH_IDENTITY_CONFLICT: 'auth.error.conflict',
  MINI_AUTH_TERMS_REQUIRED: 'auth.error.terms',
  MINI_AUTH_ACCOUNT_EXISTS: 'auth.error.accountExists',
  MINI_AUTH_EMAIL_VERIFICATION_INVALID: 'auth.error.emailVerification',
  MINI_AUTH_EMAIL_VERIFICATION_DISABLED: 'auth.error.emailVerificationDisabled',
  MINI_AUTH_UPSTREAM_UNAVAILABLE: 'auth.error.unavailable',
  MINI_AUTH_REGISTRATION_DISABLED: 'auth.error.registrationDisabled',
  MINI_AUTH_ACCOUNT_UNAVAILABLE: 'auth.error.accountUnavailable',
  MINI_AUTH_PASSWORD_LOGIN_DISABLED: 'auth.error.passwordLoginDisabled',
  MINI_AUTH_PHONE_CODE_INVALID: 'auth.error.phoneCodeInvalid',
  MINI_AUTH_PHONE_CONFLICT: 'auth.error.phoneConflict',
  MINI_AUTH_PHONE_REQUIRED: 'wechat.platformRequiredHint',
  MINI_AUTH_PHONE_ACCOUNT_NOT_FOUND: 'wechat.platformRequiredHint',
  MINI_AUTH_PROFILE_INVALID: 'wechat.profileError',
}

export default function ProfilePage() {
  const { t } = useTranslation()
  const [session, setSession] = useState<MiniAuthSession | null>(() =>
    getMiniAuthSession()
  )
  const [platformRequired, setPlatformRequired] = useState(false)
  const [binding, setBinding] = useState<MiniBindingRequiredData | null>(null)
  const [accountLoginOpen, setAccountLoginOpen] = useState(false)
  const [smsOpen, setSmsOpen] = useState(false)
  const [smsFlow, setSmsFlow] = useState('')
  const [smsAvailable, setSmsAvailable] = useState(false)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [twoFactorCode, setTwoFactorCode] = useState('')
  const [twoFactorRequired, setTwoFactorRequired] = useState(false)
  const [busy, setBusy] = useState(false)
  const [errorKey, setErrorKey] = useState('')

  usePageTitle('nav.profile')
  useDidShow(() => setSession(getMiniAuthSession()))

  const showError = (error: unknown) => {
    if (error instanceof ApiRequestError && error.code) {
      if (error.code === 'MINI_AUTH_2FA_REQUIRED') {
        setTwoFactorRequired(true)
        const details = error.details as
          | { flow_token?: string; sms_available?: boolean }
          | undefined
        setSmsFlow(details?.flow_token ?? '')
        setSmsAvailable(!!details?.sms_available)
      }
      setErrorKey(authErrorKeys[error.code] ?? 'auth.error.generic')
      return
    }
    setErrorKey('auth.error.generic')
  }

  const finishLogin = (
    result: Awaited<ReturnType<typeof loginWithWeChatPhone>>
  ) => {
    if (result.kind === 'authenticated') {
      setSession(result.session)
      setPassword('')
      setTwoFactorCode('')
      setSmsFlow('')
      setSmsAvailable(false)
      setBinding(null)
      setAccountLoginOpen(false)
    } else {
      setBinding(result.binding)
      setAccountLoginOpen(false)
    }
  }

  const startPhoneLogin = async (phoneCode?: string) => {
    if (!phoneCode) {
      setAccountLoginOpen(true)
      setErrorKey('auth.error.phoneUnavailable')
      return
    }
    setBusy(true)
    setErrorKey('')
    setPlatformRequired(false)
    setBinding(null)
    try {
      finishLogin(await loginWithWeChatPhone(phoneCode))
    } catch (error) {
      if (
        error instanceof ApiRequestError &&
        error.code === 'MINI_AUTH_PHONE_ACCOUNT_NOT_FOUND'
      ) {
        setAccountLoginOpen(false)
        setPlatformRequired(true)
      } else {
        setAccountLoginOpen(true)
        showError(error)
      }
    } finally {
      setBusy(false)
    }
  }

  const submitAccountLogin = async () => {
    setBusy(true)
    setErrorKey('')
    try {
      const authenticated = await loginWithPassword({
        username,
        password,
        twoFactorCode,
      })
      setSession(authenticated)
      setAccountLoginOpen(false)
      setPassword('')
      setTwoFactorCode('')
    } catch (error) {
      showError(error)
    } finally {
      setBusy(false)
    }
  }

  const submitBinding = async (input: WeChatConfirmationInput) => {
    if (!binding?.matched_account) return
    setBusy(true)
    setErrorKey('')
    try {
      const authenticated = await bindMiniAppAccount({
        ...input,
        flowToken: binding.flow_token,
        confirmOnly: true,
      })
      setSession(authenticated)
      setBinding(null)
      setPassword('')
      setTwoFactorCode('')
    } catch (error) {
      showError(error)
    } finally {
      setBusy(false)
    }
  }

  const logout = async () => {
    setBusy(true)
    setErrorKey('')
    try {
      await logoutMiniApp()
      setSession(null)
    } catch (error) {
      showError(error)
      setSession(null)
    } finally {
      setBusy(false)
    }
  }

  if (session) {
    return (
      <PageShell
        compact
        eyebrow={t('auth.eyebrow')}
        title={t('profile.title')}
        description={t('profile.description')}
      >
        <View className='profile-card profile-account'>
          <Text className='profile-card__label'>{t('auth.signedInAs')}</Text>
          <Text className='profile-card__username'>
            {session.user.display_name || session.user.username}
          </Text>
          <Text className='profile-card__hint'>@{session.user.username}</Text>
        </View>
        <WeChatBindingPanel editable onUnlinked={() => setSession(null)} />
        <View className='profile-card profile-services'>
          <View className='profile-nav'>
            <Button
              className='profile-nav__item'
              onClick={() => Taro.navigateTo({ url: '/pages/account/index' })}
            >
              {t('account.title')}
            </Button>
            <Button
              className='profile-nav__item'
              onClick={() => Taro.navigateTo({ url: '/pages/wallet/index' })}
            >
              {t('account.wallet')}
            </Button>
            <Button
              className='profile-nav__item'
              onClick={() => Taro.navigateTo({ url: '/pages/keys/index' })}
            >
              {t('keys.manage')}
            </Button>
            <Button
              className='profile-nav__item'
              onClick={() => Taro.navigateTo({ url: '/pages/sessions/index' })}
            >
              {t('account.sessions')}
            </Button>
            <Button
              className='profile-nav__item'
              onClick={() => Taro.navigateTo({ url: '/pages/benefits/index' })}
            >
              {t('account.benefits')}
            </Button>
            <Button
              className='profile-nav__item'
              onClick={() => Taro.navigateTo({ url: '/pages/privacy/index' })}
            >
              {t('account.privacy')}
            </Button>
          </View>
          {errorKey ? (
            <Text className='profile-card__error'>{t(errorKey)}</Text>
          ) : null}
          <Button
            className='profile-button profile-button--secondary'
            disabled={busy}
            onClick={logout}
          >
            {busy ? t('auth.busy') : t('auth.logout')}
          </Button>
        </View>
      </PageShell>
    )
  }

  if (platformRequired || (binding && !binding.matched_account?.username)) {
    return (
      <PageShell
        eyebrow={t('auth.eyebrow')}
        title={t('wechat.platformRequiredTitle')}
        description={t('wechat.platformRequiredHint')}
      >
        <View className='profile-card'>
          <Text className='profile-card__hint'>
            {t('wechat.platformRequiredSteps')}
          </Text>
          <Button
            className='profile-button profile-button--secondary'
            onClick={() => {
              setPlatformRequired(false)
              setBinding(null)
              setErrorKey('')
            }}
          >
            {t('auth.accountLoginBack')}
          </Button>
        </View>
      </PageShell>
    )
  }

  if (smsOpen) {
    return (
      <PageShell
        eyebrow={t('auth.eyebrow')}
        title={t('phone.login')}
        description={t('phone.description')}
      >
        <SmsLogin
          flowToken={smsFlow || undefined}
          onLogin={(authenticated) => {
            setSession(authenticated)
            setSmsOpen(false)
            setSmsFlow('')
            setPassword('')
            setTwoFactorCode('')
            setBinding(null)
            setAccountLoginOpen(false)
          }}
          onCancel={() => setSmsOpen(false)}
        />
      </PageShell>
    )
  }

  if (accountLoginOpen && !binding) {
    return (
      <PageShell
        eyebrow={t('auth.eyebrow')}
        title={t('auth.accountLoginTitle')}
        description={t('auth.accountLoginDescription')}
      >
        <View className='profile-card profile-form'>
          <Input
            className='profile-input'
            value={username}
            maxlength={50}
            placeholder={t('auth.username')}
            onInput={(event) => {
              setUsername(event.detail.value)
              setSmsFlow('')
              setSmsAvailable(false)
            }}
          />
          <Input
            className='profile-input'
            value={password}
            password
            maxlength={128}
            placeholder={t('auth.password')}
            onInput={(event) => {
              setPassword(event.detail.value)
              setSmsFlow('')
              setSmsAvailable(false)
            }}
          />
          {twoFactorRequired ? (
            <Input
              className='profile-input'
              value={twoFactorCode}
              maxlength={32}
              placeholder={t('auth.twoFactorCode')}
              onInput={(event) => setTwoFactorCode(event.detail.value)}
            />
          ) : null}
          {smsAvailable ? (
            <Button
              className='profile-link'
              disabled={busy}
              onClick={() => setSmsOpen(true)}
            >
              {t('phone.login')}
            </Button>
          ) : null}
          {errorKey ? (
            <Text className='profile-card__error'>{t(errorKey)}</Text>
          ) : null}
          <Button
            className='profile-button'
            disabled={busy || !username.trim() || !password}
            onClick={submitAccountLogin}
          >
            {busy ? t('auth.busy') : t('auth.accountLoginSubmit')}
          </Button>
          <Button
            className='profile-link'
            onClick={() => {
              setAccountLoginOpen(false)
              setErrorKey('')
            }}
          >
            {t('auth.accountLoginBack')}
          </Button>
        </View>
      </PageShell>
    )
  }

  if (!binding) {
    return (
      <PageShell
        eyebrow={t('auth.eyebrow')}
        title={t('auth.signedOutTitle')}
        description={t('auth.signedOutDescription')}
      >
        <View className='profile-card'>
          {errorKey ? (
            <Text className='profile-card__error'>{t(errorKey)}</Text>
          ) : null}
          <Button
            className='profile-button'
            disabled={busy}
            openType='getPhoneNumber'
            onGetPhoneNumber={(event) => startPhoneLogin(event.detail.code)}
          >
            {busy ? t('auth.signingIn') : t('auth.wechatPhoneLogin')}
          </Button>
          <Button
            className='profile-button profile-button--secondary'
            disabled={busy}
            onClick={() => {
              setAccountLoginOpen(true)
              setErrorKey('')
            }}
          >
            {t('auth.existingWeChatLogin')}
          </Button>
        </View>
      </PageShell>
    )
  }

  return (
    <PageShell
      eyebrow={t('auth.eyebrow')}
      title={t('wechat.confirm')}
      description={t('wechat.connectionHint')}
    >
      <WeChatConnectionConfirmation
        key={binding.flow_token}
        binding={binding}
        busy={busy}
        errorKey={errorKey}
        onConfirm={submitBinding}
        onCancel={() => {
          setBinding(null)
          setErrorKey('')
          setPassword('')
          setTwoFactorCode('')
          setTwoFactorRequired(false)
        }}
      />
    </PageShell>
  )
}
