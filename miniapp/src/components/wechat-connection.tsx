import { Button, Checkbox, Image, Input, Text, View } from '@tarojs/components'
import Taro from '@tarojs/taro'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { MiniBindingRequiredData } from '@/auth/client'

import './wechat-connection.scss'

export interface WeChatConfirmationInput {
  acceptTerms: boolean
  avatarPath?: string
  nickname?: string
}

export function WeChatConnectionConfirmation(props: {
  binding: MiniBindingRequiredData
  busy: boolean
  errorKey: string
  onConfirm: (input: WeChatConfirmationInput) => void
  onCancel: () => void
}) {
  const { t } = useTranslation()
  const [avatar, setAvatar] = useState('')
  const [nickname, setNickname] = useState('')
  const nicknameValue = useRef('')
  const [nicknameFocused, setNicknameFocused] = useState(false)
  const [acceptTerms, setAcceptTerms] = useState(false)
  const account = props.binding.matched_account
  if (!account?.username) return null
  const accountName = account.display_name || account.username
  const legalRequired =
    props.binding.user_agreement_enabled || props.binding.privacy_policy_enabled
  const avatarHash = Array.from(account.username).reduce(
    (hash, character) => (hash * 31 + character.charCodeAt(0)) >>> 0,
    0
  )

  return (
    <View className='profile-card profile-form wechat-connection'>
      <View className='wechat-connection__identities'>
        <View className='wechat-connection__identity'>
          <View className='wechat-connection__avatar wechat-connection__avatar--wechat'>
            {avatar ? (
              <Image
                className='wechat-connection__image'
                src={avatar}
                mode='aspectFill'
              />
            ) : (
              <View className='wechat-connection__wechat-icon'>
                <View className='wechat-connection__bubble wechat-connection__bubble--first'>
                  <View className='wechat-connection__eye' />
                  <View className='wechat-connection__eye' />
                </View>
                <View className='wechat-connection__bubble wechat-connection__bubble--second'>
                  <View className='wechat-connection__eye' />
                  <View className='wechat-connection__eye' />
                </View>
              </View>
            )}
          </View>
          <Text className='wechat-connection__label'>
            {t('wechat.currentIdentity')}
          </Text>
          <Text className='wechat-connection__name'>
            {nickname.trim() || t('wechat.currentWeChat')}
          </Text>
        </View>
        <View
          className='wechat-connection__bridge'
          ariaLabel={t('wechat.pendingConnection')}
        >
          <View className='wechat-connection__line' />
          <Text className='wechat-connection__link-icon'>⇄</Text>
          <View className='wechat-connection__line' />
        </View>
        <View className='wechat-connection__identity'>
          <View
            className='wechat-connection__avatar'
            style={{
              backgroundColor: `hsl(${avatarHash % 360} ${54 + (avatarHash % 8)}% ${52 + ((avatarHash >> 4) % 8)}%)`,
              color: '#ffffff',
            }}
          >
            <Text>{account.username.trim().charAt(0).toUpperCase()}</Text>
          </View>
          <Text className='wechat-connection__label'>
            {t('wechat.platformAccount')}
          </Text>
          <Text className='wechat-connection__name'>{accountName}</Text>
          <Text className='wechat-connection__detail'>{account.username}</Text>
        </View>
      </View>
      <View className='wechat-connection__match'>
        <Text className='wechat-connection__match-title'>
          {t('wechat.phoneMatched')}
        </Text>
        <Text className='wechat-connection__detail'>{account.phone_hint}</Text>
      </View>
      <Text className='profile-card__hint'>{t('wechat.confirmEffect')}</Text>
      <View className='wechat-connection__profile'>
        <Text className='wechat-connection__profile-title'>
          {t('wechat.displayProfile')}
        </Text>
        <Text className='profile-card__hint'>
          {t('wechat.displayProfileHint')}
        </Text>
        <Button
          className='wechat-connection__choose-avatar'
          openType='chooseAvatar'
          disabled={props.busy}
          onChooseAvatar={(event) => setAvatar(event.detail.avatarUrl)}
        >
          {avatar ? t('wechat.changeAvatar') : t('wechat.chooseAvatar')}
        </Button>
        <Text className='wechat-connection__nickname-label'>
          {t('wechat.nicknameLabel')}
        </Text>
        <Button
          className='wechat-connection__choose-avatar'
          disabled={props.busy}
          onClick={() => setNicknameFocused(true)}
        >
          {t('wechat.useWeChatNickname')}
        </Button>
        <Text className='profile-card__hint'>
          {t('wechat.nicknameAuthorizationHint')}
        </Text>
        <Input
          className='profile-input wechat-connection__nickname'
          type='nickname'
          focus={nicknameFocused}
          value={nickname}
          maxlength={64}
          disabled={props.busy}
          placeholder={t('wechat.nicknamePlaceholder')}
          ariaLabel={t('wechat.nicknameLabel')}
          onInput={(event) => {
            nicknameValue.current = event.detail.value
            setNickname(event.detail.value)
          }}
          onFocus={() => setNicknameFocused(true)}
          onBlur={(event) => {
            // Native nickname selection can update the value without onInput.
            nicknameValue.current = event.detail.value
            setNickname(event.detail.value)
            setNicknameFocused(false)
          }}
          onConfirm={(event) => {
            nicknameValue.current = event.detail.value
            setNickname(event.detail.value)
            setNicknameFocused(false)
          }}
        />
      </View>
      {legalRequired ? (
        <View
          className='profile-consent'
          onClick={() => {
            if (!props.busy) setAcceptTerms((value) => !value)
          }}
        >
          <Checkbox
            value='accepted'
            checked={acceptTerms}
            disabled={props.busy}
            color='#4f46e5'
          />
          <Text className='profile-consent__text'>{t('auth.acceptTerms')}</Text>
        </View>
      ) : null}
      {legalRequired ? (
        <View className='profile-legal-links'>
          {props.binding.user_agreement_enabled ? (
            <Button
              className='profile-link'
              onClick={() =>
                Taro.navigateTo({ url: '/pages/legal/index?kind=agreement' })
              }
            >
              {t('auth.viewAgreement')}
            </Button>
          ) : null}
          {props.binding.privacy_policy_enabled ? (
            <Button
              className='profile-link'
              onClick={() =>
                Taro.navigateTo({ url: '/pages/legal/index?kind=privacy' })
              }
            >
              {t('auth.viewPrivacy')}
            </Button>
          ) : null}
        </View>
      ) : null}
      {props.errorKey ? (
        <Text className='profile-card__error'>{t(props.errorKey)}</Text>
      ) : null}
      <Button
        className='profile-button'
        disabled={props.busy || (legalRequired && !acceptTerms)}
        onClick={() =>
          props.onConfirm({
            acceptTerms,
            avatarPath: avatar || undefined,
            nickname: nicknameValue.current.trim() || undefined,
          })
        }
      >
        {props.busy ? t('auth.busy') : t('wechat.confirmAndSignIn')}
      </Button>
      <Button
        className='profile-link'
        disabled={props.busy}
        onClick={props.onCancel}
      >
        {t('wechat.cancelConnection')}
      </Button>
    </View>
  )
}
