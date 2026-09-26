import { useEffect, useMemo, useRef, useState } from 'react'

import { Button, Picker, ScrollView, Text, Textarea, View } from '@tarojs/components'
import Taro, { useDidHide, useDidShow } from '@tarojs/taro'
import { useTranslation } from 'react-i18next'

import { getUserGroups, getUserModels } from '@/api/models'
import { startStreamingChat, type StreamingChat } from '@/api/playground'
import { getMiniAuthSession } from '@/auth/session'
import { PageShell } from '@/components/page-shell'
import { usePageTitle } from '@/hooks/use-page-title'
import {
  clearConversation,
  loadConversation,
  saveConversation,
  type ChatMessage,
} from '@/playground/storage'

import './index.scss'

function messageId(role: string) {
  return `${role}-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

export default function PlaygroundPage() {
  const { t } = useTranslation()
  const draft = useMemo(loadConversation, [])
  const [signedIn, setSignedIn] = useState(() => !!getMiniAuthSession())
  const [groups, setGroups] = useState<string[]>([])
  const [models, setModels] = useState<string[]>([])
  const [group, setGroup] = useState(draft.group)
  const [model, setModel] = useState(draft.model)
  const [messages, setMessages] = useState<ChatMessage[]>(draft.messages)
  const [input, setInput] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [loadingOptions, setLoadingOptions] = useState(signedIn)
  const [errorKey, setErrorKey] = useState('')
  const activeStream = useRef<StreamingChat | null>(null)
  usePageTitle('nav.playground')

  useDidShow(() => setSignedIn(!!getMiniAuthSession()))
  useDidHide(() => activeStream.current?.abort())

  useEffect(() => {
    if (!signedIn) return
    getUserGroups()
      .then((data) => {
        const nextGroups = Object.keys(data)
        setGroups(nextGroups)
        setGroup((current) => current || nextGroups[0] || '')
      })
      .catch(() => setErrorKey('playground.optionsError'))
      .finally(() => setLoadingOptions(false))
  }, [signedIn])

  useEffect(() => {
    if (!signedIn || !group) return
    setLoadingOptions(true)
    getUserModels(group)
      .then((items) => {
        setModels(items)
        setModel((current) => (items.includes(current) ? current : items[0] || ''))
      })
      .catch(() => setErrorKey('playground.optionsError'))
      .finally(() => setLoadingOptions(false))
  }, [group, signedIn])

  useEffect(() => {
    const timer = setTimeout(() => {
      saveConversation({ group, messages, model, version: 1 })
    }, 300)
    return () => clearTimeout(timer)
  }, [group, messages, model])

  useEffect(() => () => activeStream.current?.abort(), [])

  const runStream = async (baseMessages: ChatMessage[]) => {
    if (!model || !group || streaming) return
    const assistant: ChatMessage = {
      content: '',
      createdAt: Date.now(),
      id: messageId('assistant'),
      role: 'assistant',
    }
    const conversation = [...baseMessages, assistant]
    setMessages(conversation)
    setStreaming(true)
    setErrorKey('')
    try {
      const stream = await startStreamingChat(model, group, baseMessages, {
        onDelta: (content) => {
          if (!content) return
          setMessages((items) =>
            items.map((item) =>
              item.id === assistant.id ? { ...item, content: item.content + content } : item
            )
          )
        },
        onDone: () => undefined,
      })
      activeStream.current = stream
      await stream.completion
    } catch (error) {
      if (!(error instanceof Error) || error.message !== 'STREAM_ABORTED') {
        setErrorKey('playground.streamError')
      }
    } finally {
      activeStream.current = null
      setStreaming(false)
    }
  }

  const send = () => {
    const content = input.trim()
    if (!content) return
    const user: ChatMessage = {
      content,
      createdAt: Date.now(),
      id: messageId('user'),
      role: 'user',
    }
    setInput('')
    void runStream([...messages, user])
  }

  const retry = () => {
    const base = messages.at(-1)?.role === 'assistant' ? messages.slice(0, -1) : messages
    if (base.at(-1)?.role === 'user') void runStream(base)
  }

  const editLast = () => {
    let index = messages.length - 1
    while (index >= 0 && messages[index].role !== 'user') index -= 1
    if (index < 0) return
    setInput(messages[index].content)
    setMessages(messages.slice(0, index))
  }

  const clear = async () => {
    const result = await Taro.showModal({ title: t('playground.clear'), content: t('playground.clearConfirm') })
    if (!result.confirm) return
    activeStream.current?.abort()
    setMessages([])
    setInput('')
    clearConversation()
  }

  if (!signedIn) {
    return (
      <PageShell title={t('playground.title')} description={t('playground.description')}>
        <Text className='mobile-empty'>{t('playground.loginRequired')}</Text>
      </PageShell>
    )
  }

  return (
    <PageShell title={t('playground.title')} description={t('playground.description')}>
      <View className='playground-options'>
        <Picker mode='selector' range={groups} value={Math.max(0, groups.indexOf(group))} onChange={(event) => setGroup(groups[Number(event.detail.value)] ?? '')}>
          <View className='mobile-picker'>{t('playground.group')}: {group || '—'}</View>
        </Picker>
        <Picker mode='selector' range={models} value={Math.max(0, models.indexOf(model))} onChange={(event) => setModel(models[Number(event.detail.value)] ?? '')}>
          <View className='mobile-picker'>{t('playground.model')}: {model || '—'}</View>
        </Picker>
      </View>
      {loadingOptions ? <Text className='mobile-muted'>{t('common.loading')}</Text> : null}
      {errorKey ? <Text className='mobile-error'>{t(errorKey)}</Text> : null}

      <ScrollView
        className='playground-chat'
        scrollY
        scrollTop={messages.reduce((total, message) => total + message.content.length * 2 + 1000, 0)}
      >
        {messages.length === 0 ? <Text className='mobile-empty'>{t('playground.empty')}</Text> : null}
        {messages.map((message) => (
          <View className={`playground-message playground-message--${message.role}`} key={message.id}>
            <Text className='playground-message__role'>
              {message.role === 'user' ? t('playground.you') : t('playground.assistant')}
            </Text>
            <Text className='playground-message__content'>
              {message.content || (streaming ? t('playground.streaming') : '')}
            </Text>
          </View>
        ))}
        <View id='playground-end' />
      </ScrollView>

      <View className='mobile-card mobile-stack'>
        <Textarea className='mobile-textarea' value={input} maxlength={8000} placeholder={t('playground.input')} onInput={(event) => setInput(event.detail.value)} />
        <View className='mobile-actions'>
          {streaming ? (
            <Button className='mobile-button mobile-button--danger' onClick={() => activeStream.current?.abort()}>{t('playground.stop')}</Button>
          ) : (
            <Button className='mobile-button' disabled={!input.trim() || !model || !group} onClick={send}>{t('playground.send')}</Button>
          )}
          <Button className='mobile-button mobile-button--secondary' disabled={streaming || messages.length === 0} onClick={retry}>{t('playground.retry')}</Button>
          <Button className='mobile-button mobile-button--secondary' disabled={streaming || messages.length === 0} onClick={editLast}>{t('playground.edit')}</Button>
          <Button className='mobile-button mobile-button--secondary' onClick={clear}>{t('playground.clear')}</Button>
        </View>
      </View>
    </PageShell>
  )
}
