import { useState } from 'react'

import { Button, Checkbox, Input, Text, View } from '@tarojs/components'
import Taro, { useDidShow } from '@tarojs/taro'
import { useTranslation } from 'react-i18next'

import { ApiRequestError } from '@/api/request'
import {
  bindMiniAppAccount,
  loginWithWeChat,
  logoutMiniApp,
  registerMiniAppAccount,
  sendMiniAppEmailVerification,
  type MiniBindingRequiredData,
} from '@/auth/client'
import { getMiniAuthSession, type MiniAuthSession } from '@/auth/session'
import { PageShell } from '@/components/page-shell'
import { usePageTitle } from '@/hooks/use-page-title'

import './index.scss'

const authErrorKeys: Record<string, string> = {
  MINI_AUTH_NOT_CONFIGURED: 'auth.error.notConfigured',
  MINI_AUTH_DISABLED: 'auth.error.notConfigured',
  MINI_AUTH_CODE_INVALID: 'auth.error.invalidCode',
  MINI_AUTH_CREDENTIALS_INVALID: 'auth.error.credentials',
  MINI_AUTH_2FA_REQUIRED: 'auth.error.twoFactorRequired',
  MINI_AUTH_2FA_INVALID: 'auth.error.twoFactorInvalid',
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
}

export default function ProfilePage() {
  const { t } = useTranslation()
  const [session, setSession] = useState<MiniAuthSession | null>(() =>
    getMiniAuthSession()
  )
  const [binding, setBinding] = useState<MiniBindingRequiredData | null>(null)
  const [mode, setMode] = useState<'bind' | 'register'>('bind')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [email, setEmail] = useState('')
  const [verificationCode, setVerificationCode] = useState('')
  const [twoFactorCode, setTwoFactorCode] = useState('')
  const [affCode, setAffCode] = useState('')
  const [acceptTerms, setAcceptTerms] = useState(false)
  const [twoFactorRequired, setTwoFactorRequired] = useState(false)
  const [verificationSent, setVerificationSent] = useState(false)
  const [busy, setBusy] = useState(false)
  const [errorKey, setErrorKey] = useState('')

  usePageTitle('nav.profile')
  useDidShow(() => setSession(getMiniAuthSession()))

  const showError = (error: unknown) => {
    if (error instanceof ApiRequestError && error.code) {
      if (error.code === 'MINI_AUTH_2FA_REQUIRED') {
        setTwoFactorRequired(true)
      }
      setErrorKey(authErrorKeys[error.code] ?? 'auth.error.generic')
      return
    }
    setErrorKey('auth.error.generic')
  }

  const startLogin = async () => {
    setBusy(true)
    setErrorKey('')
    try {
      const result = await loginWithWeChat()
      if (result.kind === 'authenticated') {
        setSession(result.session)
        setBinding(null)
      } else {
        setBinding(result.binding)
        setMode(result.binding.password_login_enabled ? 'bind' : 'register')
      }
    } catch (error) {
      showError(error)
    } finally {
      setBusy(false)
    }
  }

  const submitBinding = async () => {
    if (!binding) {
      return
    }
    setBusy(true)
    setErrorKey('')
    try {
      const authenticated =
        mode === 'bind'
          ? await bindMiniAppAccount({
              flowToken: binding.flow_token,
              username,
              password,
              twoFactorCode,
              acceptTerms,
            })
          : await registerMiniAppAccount({
              flowToken: binding.flow_token,
              username,
              password,
              email,
              verificationCode,
              affCode,
              acceptTerms,
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

  const sendVerification = async () => {
    if (!binding) {
      return
    }
    setBusy(true)
    setErrorKey('')
    setVerificationSent(false)
    try {
      await sendMiniAppEmailVerification(binding.flow_token, email)
      setVerificationSent(true)
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
        eyebrow={t('auth.eyebrow')}
        title={t('profile.title')}
        description={t('profile.description')}
      >
        <View className='profile-card'>
          <Text className='profile-card__label'>{t('auth.signedInAs')}</Text>
          <Text className='profile-card__username'>
            {session.user.display_name || session.user.username}
          </Text>
          <Text className='profile-card__hint'>{t('auth.sessionReady')}</Text>
          <View className='profile-nav'>
            <Button className='profile-nav__item' onClick={() => Taro.navigateTo({ url: '/pages/account/index' })}>
              {t('account.title')}
            </Button>
            <Button className='profile-nav__item' onClick={() => Taro.navigateTo({ url: '/pages/wallet/index' })}>
              {t('account.wallet')}
            </Button>
            <Button className='profile-nav__item' onClick={() => Taro.navigateTo({ url: '/pages/keys/index' })}>
              {t('keys.manage')}
            </Button>
            <Button className='profile-nav__item' onClick={() => Taro.navigateTo({ url: '/pages/sessions/index' })}>
              {t('account.sessions')}
            </Button>
            <Button className='profile-nav__item' onClick={() => Taro.navigateTo({ url: '/pages/benefits/index' })}>
              {t('account.benefits')}
            </Button>
            <Button className='profile-nav__item' onClick={() => Taro.navigateTo({ url: '/pages/privacy/index' })}>
              {t('account.privacy')}
            </Button>
          </View>
          {errorKey ? (
            <Text className='profile-card__error'>{t(errorKey)}</Text>
          ) : null}
          <Button className='profile-button profile-button--secondary' disabled={busy} onClick={logout}>
            {busy ? t('auth.busy') : t('auth.logout')}
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
          <Button className='profile-button' disabled={busy} onClick={startLogin}>
            {busy ? t('auth.signingIn') : t('auth.wechatLogin')}
          </Button>
        </View>
      </PageShell>
    )
  }

  if (!binding.password_login_enabled && !binding.registration_enabled) {
    return (
      <PageShell
        eyebrow={t('auth.eyebrow')}
        title={t('auth.bindTitle')}
        description={t('auth.bindDescription')}
      >
        <View className='profile-card'>
          <Text className='profile-card__error'>{t('auth.error.noAvailableMethod')}</Text>
          <Button className='profile-link' onClick={() => setBinding(null)}>
            {t('auth.back')}
          </Button>
        </View>
      </PageShell>
    )
  }

  return (
    <PageShell
      eyebrow={t('auth.eyebrow')}
      title={t('auth.bindTitle')}
      description={t('auth.bindDescription')}
    >
      <View className='profile-mode'>
        {binding.password_login_enabled ? (
          <Button
            className={`profile-mode__button ${mode === 'bind' ? 'profile-mode__button--active' : ''}`}
            onClick={() => setMode('bind')}
          >
            {t('auth.existingAccount')}
          </Button>
        ) : null}
        {binding.registration_enabled ? (
          <Button
            className={`profile-mode__button ${mode === 'register' ? 'profile-mode__button--active' : ''}`}
            onClick={() => setMode('register')}
          >
            {t('auth.createAccount')}
          </Button>
        ) : null}
      </View>

      <View className='profile-card profile-form'>
        <Input
          className='profile-input'
          value={username}
          maxlength={20}
          placeholder={t('auth.username')}
          onInput={(event) => setUsername(event.detail.value)}
        />
        <Input
          className='profile-input'
          value={password}
          password
          maxlength={20}
          placeholder={t('auth.password')}
          onInput={(event) => setPassword(event.detail.value)}
        />
        {mode === 'bind' && twoFactorRequired ? (
          <Input
            className='profile-input'
            value={twoFactorCode}
            maxlength={32}
            placeholder={t('auth.twoFactorCode')}
            onInput={(event) => setTwoFactorCode(event.detail.value)}
          />
        ) : null}
        {mode === 'register' && binding.email_verification_enabled ? (
          <>
            <Input
              className='profile-input'
              value={email}
              maxlength={50}
              placeholder={t('auth.email')}
              onInput={(event) => setEmail(event.detail.value)}
            />
            <Input
              className='profile-input'
              value={verificationCode}
              maxlength={16}
              placeholder={t('auth.verificationCode')}
              onInput={(event) => setVerificationCode(event.detail.value)}
            />
            <Button
              className='profile-button profile-button--secondary'
              disabled={busy || !email}
              onClick={sendVerification}
            >
              {t('auth.sendVerificationCode')}
            </Button>
            {verificationSent ? (
              <Text className='profile-card__hint'>{t('auth.verificationCodeSent')}</Text>
            ) : null}
          </>
        ) : null}
        {mode === 'register' ? (
          <Input
            className='profile-input'
            value={affCode}
            maxlength={32}
            placeholder={t('auth.affCode')}
            onInput={(event) => setAffCode(event.detail.value)}
          />
        ) : null}

        {binding.user_agreement_enabled || binding.privacy_policy_enabled ? (
          <View className='profile-consent' onClick={() => setAcceptTerms((accepted) => !accepted)}>
            <Checkbox value='accepted' checked={acceptTerms} color='#4f46e5' />
            <Text className='profile-consent__text'>{t('auth.acceptTerms')}</Text>
          </View>
        ) : null}
        <View className='profile-legal-links'>
          {binding.user_agreement_enabled ? (
            <Button
              className='profile-link'
              onClick={() => Taro.navigateTo({ url: '/pages/legal/index?kind=agreement' })}
            >
              {t('auth.viewAgreement')}
            </Button>
          ) : null}
          {binding.privacy_policy_enabled ? (
            <Button
              className='profile-link'
              onClick={() => Taro.navigateTo({ url: '/pages/legal/index?kind=privacy' })}
            >
              {t('auth.viewPrivacy')}
            </Button>
          ) : null}
        </View>

        {errorKey ? <Text className='profile-card__error'>{t(errorKey)}</Text> : null}
        <Button className='profile-button' disabled={busy} onClick={submitBinding}>
          {busy
            ? t('auth.busy')
            : mode === 'bind'
              ? t('auth.bind')
              : t('auth.register')}
        </Button>
        <Button className='profile-link' onClick={() => setBinding(null)}>
          {t('auth.back')}
        </Button>
      </View>
    </PageShell>
  )
}
