import { memo, useEffect, useRef, useState } from 'react'

import { Button, Image, Picker, ScrollView, Text, Textarea, View } from '@tarojs/components'
import Taro, { useDidHide, useDidShow } from '@tarojs/taro'
import { useTranslation } from 'react-i18next'

import { getUserGroups, getUserModels } from '@/api/models'
import { getPlaygroundMedia, synthesizePlaygroundSpeech, transcribePlaygroundAudio, type PlaygroundMedia } from '@/api/playground-media'
import { startStreamingChat, type StreamingChat } from '@/api/playground'
import { getMiniAuthSession } from '@/auth/session'
import { PageShell } from '@/components/page-shell'
import { usePageTitle } from '@/hooks/use-page-title'
import {
  conversationTitle,
  conversationHistory,
  createConversation,
  loadConversationStore,
  removeConversationImages,
  saveConversationStore,
  type ChatMessage,
} from '@/playground/storage'
import { parseChatInline, parseChatMarkdown } from '@/playground/format-message'

import './index.scss'

function messageId(role: string) {
  return `${role}-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

function removeLocalSpeech(path: string) {
  if (path) Taro.getFileSystemManager().unlink({ filePath: path })
}

const ChatAnswer = memo(function ChatAnswer({ content }: { content: string }) {
  return (
    <View className='playground-markdown'>
      {parseChatMarkdown(content).map((block, blockIndex) => {
        if (block.kind === 'rule') return <View className='playground-markdown__rule' key={blockIndex} />
        if (block.kind === 'list') {
          return (
            <View className='playground-markdown__list' key={blockIndex}>
              {block.items?.map((item, itemIndex) => (
                <View className='playground-markdown__list-item' key={itemIndex} style={{ marginLeft: `${item.depth * 24}rpx` }}>
                  <Text className='playground-markdown__marker'>{item.marker}</Text>
                  <Text className='playground-markdown__body'>
                    {parseChatInline(item.text).map((segment, segmentIndex) => (
                      <Text className={`playground-markdown__${segment.kind}`} key={segmentIndex}>{segment.text}</Text>
                    ))}
                  </Text>
                </View>
              ))}
            </View>
          )
        }
        if (block.kind === 'code') return <Text className='playground-markdown__code-block' key={blockIndex}>{block.text}</Text>
        return (
          <Text className={`playground-markdown__${block.kind}`} key={blockIndex}>
            {parseChatInline(block.text || '').map((segment, segmentIndex) => (
              <Text className={`playground-markdown__${segment.kind}`} key={segmentIndex}>{segment.text}</Text>
            ))}
          </Text>
        )
      })}
    </View>
  )
})

export default function PlaygroundPage() {
  const { t } = useTranslation()
  const [store, setStore] = useState(loadConversationStore)
  const activeConversation = store.conversations.find((item) => item.id === store.activeId) ?? store.conversations[0]
  const group = activeConversation.group
  const model = activeConversation.model
  const messages = activeConversation.messages
  const [signedIn, setSignedIn] = useState(() => !!getMiniAuthSession())
  const [groups, setGroups] = useState<string[]>([])
  const [models, setModels] = useState<string[]>([])
  const [input, setInput] = useState('')
  const [imagePath, setImagePath] = useState('')
  const [media, setMedia] = useState<PlaygroundMedia | null>(null)
  const [recording, setRecording] = useState(false)
  const [transcribing, setTranscribing] = useState(false)
  const [playingId, setPlayingId] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [loadingOptions, setLoadingOptions] = useState(signedIn)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [historyOpen, setHistoryOpen] = useState(false)
  const historyConversations = historyOpen ? conversationHistory(store.conversations) : []
  const [moreActionsOpen, setMoreActionsOpen] = useState(false)
  const [errorKey, setErrorKey] = useState('')
  const activeStream = useRef<StreamingChat | null>(null)
  const recorder = useRef<ReturnType<typeof Taro.getRecorderManager> | null>(null)
  const recorderConfig = useRef<{ group: string; model: string; transcriptionModel: string } | null>(null)
  const audio = useRef<ReturnType<typeof Taro.createInnerAudioContext> | null>(null)
  const speechGeneration = useRef(0)
  const speechPath = useRef('')
  const draftGeneration = useRef(0)
  const streamGeneration = useRef(0)
  usePageTitle('nav.playground')

  useDidShow(() => {
    const authenticated = !!getMiniAuthSession()
    setSignedIn(authenticated)
    if (authenticated) setStore(loadConversationStore())
  })
  useDidHide(() => {
    recorderConfig.current = null
    if (recording) recorder.current?.stop()
    speechGeneration.current += 1
    audio.current?.stop()
    removeLocalSpeech(speechPath.current)
    speechPath.current = ''
    setPlayingId('')
    activeStream.current?.abort()
    streamGeneration.current += 1
    setStreaming(false)
    if (signedIn && getMiniAuthSession()) saveConversationStore(store)
  })

  useEffect(() => {
    if (!signedIn) return
    let cancelled = false
    getUserGroups()
      .then((data) => {
        if (cancelled) return
        const nextGroups = Object.keys(data)
        setGroups(nextGroups)
        setStore((current) => ({
          ...current,
          conversations: current.conversations.map((item) => {
            if (item.id !== current.activeId || nextGroups.includes(item.group)) return item
            return { ...item, group: nextGroups[0] || '', model: '' }
          }),
        }))
      })
      .catch(() => { if (!cancelled) setErrorKey('playground.optionsError') })
      .finally(() => { if (!cancelled) setLoadingOptions(false) })
    return () => { cancelled = true }
  }, [signedIn])

  useEffect(() => {
    if (!signedIn || !group) return
    setLoadingOptions(true)
    let cancelled = false
    const conversationId = store.activeId
    getUserModels(group)
      .then((items) => {
        if (cancelled) return
        setModels(items)
        setStore((current) => ({
          ...current,
          conversations: current.conversations.map((item) => item.id === conversationId
            ? { ...item, model: items.includes(item.model) ? item.model : items[0] || '' }
            : item),
        }))
      })
      .catch(() => { if (!cancelled) setErrorKey('playground.optionsError') })
      .finally(() => { if (!cancelled) setLoadingOptions(false) })
    return () => { cancelled = true }
  }, [group, signedIn, store.activeId])

  useEffect(() => {
    if (!signedIn || !group) { setMedia(null); return }
    setMedia(null)
    let cancelled = false
    getPlaygroundMedia(group)
      .then((settings) => { if (!cancelled) setMedia(settings) })
      .catch(() => { if (!cancelled) setMedia(null) })
    return () => { cancelled = true }
  }, [group, signedIn])

  useEffect(() => {
    if (!signedIn) return
    const timer = setTimeout(() => {
      if (getMiniAuthSession() && !saveConversationStore(store)) setErrorKey('playground.storageError')
    }, 300)
    return () => clearTimeout(timer)
  }, [signedIn, store])

  useEffect(() => () => {
    activeStream.current?.abort()
    recorderConfig.current = null
    recorder.current?.stop()
    speechGeneration.current += 1
    audio.current?.destroy()
    removeLocalSpeech(speechPath.current)
    speechPath.current = ''
    streamGeneration.current += 1
  }, [])

  const runStream = async (baseMessages: ChatMessage[]) => {
    if (!model || !group || streaming) return
    const conversationId = store.activeId
    const generation = ++streamGeneration.current
    const assistant: ChatMessage = {
      content: '',
      createdAt: Date.now(),
      id: messageId('assistant'),
      role: 'assistant',
    }
    const conversation = [...baseMessages, assistant]
    setStore((current) => ({
      ...current,
      conversations: current.conversations.map((item) => item.id === conversationId
        ? { ...item, messages: conversation, updatedAt: Date.now() }
        : item),
    }))
    setStreaming(true)
    setErrorKey('')
    try {
      const stream = await startStreamingChat(model, group, baseMessages, {
        onDelta: (content) => {
          if (!content) return
          if (streamGeneration.current !== generation) return
          setStore((current) => ({
            ...current,
            conversations: current.conversations.map((item) => item.id === conversationId
              ? {
                  ...item,
                  messages: item.messages.map((message) => message.id === assistant.id
                    ? { ...message, content: message.content + content } : message),
                  updatedAt: Date.now(),
                }
              : item),
          }))
        },
        onDone: () => undefined,
      }, !!media?.image_input_models.includes(model))
      if (streamGeneration.current !== generation) {
        stream.abort()
        await stream.completion
        return
      }
      activeStream.current = stream
      await stream.completion
    } catch (error) {
      if (streamGeneration.current === generation &&
        (!(error instanceof Error) || error.message !== 'STREAM_ABORTED')) {
        setErrorKey('playground.streamError')
      }
    } finally {
      if (streamGeneration.current === generation) {
        activeStream.current = null
        setStreaming(false)
      }
    }
  }

  const submitMessage = (draftText: string) => {
    const content = draftText.trim() || (imagePath ? t('playground.imagePrompt') : '')
    if (!content || !model || !group || loadingOptions || streaming) return
    if (imagePath && !media?.image_input_models.includes(model)) { setErrorKey('playground.imageUnavailable'); return }
    const user: ChatMessage = {
      content,
      createdAt: Date.now(),
      id: messageId('user'),
      imagePath: imagePath || undefined,
      role: 'user',
    }
    setInput('')
    setImagePath('')
    void runStream([...messages, user])
  }

  const chooseImage = async () => {
    try {
      const result = await Taro.chooseImage({ count: 1, sizeType: ['compressed'], sourceType: ['album', 'camera'] })
      const image = result.tempFiles[0]
      if (!image || image.size > 1024 * 1024) { setErrorKey('playground.imageTooLarge'); return }
      const saved = await Taro.saveFile({ tempFilePath: image.path })
      if (!('savedFilePath' in saved)) throw new Error('IMAGE_SAVE_FAILED')
      if (imagePath) removeConversationImages([{ content: '', createdAt: 0, id: '', imagePath, role: 'user' }])
      setImagePath(saved.savedFilePath)
      setErrorKey('')
    } catch (error) { if (!String(error).includes('cancel')) setErrorKey('playground.imageError') }
  }

  const toggleRecording = () => {
    if (recording) { recorder.current?.stop(); return }
    if (!media?.transcription_model || !media.voice_input_models.includes(model)) return
    if (!recorder.current) {
      const manager = Taro.getRecorderManager()
      manager.onStop(({ tempFilePath }) => {
        setRecording(false)
        const config = recorderConfig.current
        recorderConfig.current = null
        if (!config) return
        const generation = draftGeneration.current
        setTranscribing(true)
        void transcribePlaygroundAudio(tempFilePath, config.group, config.model, config.transcriptionModel)
          .then((text) => { if (generation === draftGeneration.current) { setInput((current) => current ? `${current} ${text}` : text); setErrorKey('') } })
          .catch(() => { if (generation === draftGeneration.current) setErrorKey('playground.transcriptionError') })
          .finally(() => { if (generation === draftGeneration.current) setTranscribing(false) })
      })
      manager.onError(() => { recorderConfig.current = null; setRecording(false); setErrorKey('playground.recordError') })
      recorder.current = manager
    }
    recorderConfig.current = { group, model, transcriptionModel: media.transcription_model }
    try {
      recorder.current.start({ duration: 60000, format: 'mp3', sampleRate: 16000, numberOfChannels: 1 })
      setRecording(true)
      setErrorKey('')
    } catch { recorderConfig.current = null; setErrorKey('playground.recordError') }
  }

  const toggleSpeech = async (message: ChatMessage) => {
    if (playingId === message.id) { speechGeneration.current += 1; audio.current?.stop(); removeLocalSpeech(speechPath.current); speechPath.current = ''; setPlayingId(''); return }
    if (!media?.speech_model || !media.speech_voice) return
    const generation = ++speechGeneration.current
    audio.current?.destroy()
    try {
      setPlayingId(message.id)
      const path = await synthesizePlaygroundSpeech(message.content.slice(0, 2000), group, model, media.speech_model, media.speech_voice)
      if (generation !== speechGeneration.current) { removeLocalSpeech(path); return }
      removeLocalSpeech(speechPath.current)
      speechPath.current = path
      const player = Taro.createInnerAudioContext()
      player.src = path
      player.onEnded(() => { setPlayingId(''); removeLocalSpeech(path); if (speechPath.current === path) speechPath.current = '' })
      player.onError(() => { setPlayingId(''); removeLocalSpeech(path); if (speechPath.current === path) speechPath.current = ''; setErrorKey('playground.speechError') })
      audio.current = player
      player.play()
    } catch { setPlayingId(''); setErrorKey('playground.speechError') }
  }

  const retry = () => {
    const base = messages.at(-1)?.role === 'assistant' ? messages.slice(0, -1) : messages
    if (base.at(-1)?.role === 'user') void runStream(base)
  }

  const stopMediaForConversationChange = () => {
    if (imagePath) removeConversationImages([{ content: '', createdAt: 0, id: '', imagePath, role: 'user' }])
    draftGeneration.current += 1
    recorderConfig.current = null
    recorder.current?.stop()
    setRecording(false)
    setTranscribing(false)
    speechGeneration.current += 1
    audio.current?.stop()
    removeLocalSpeech(speechPath.current)
    speechPath.current = ''
    setPlayingId('')
  }

  const editLast = () => {
    let index = messages.length - 1
    while (index >= 0 && messages[index].role !== 'user') index -= 1
    if (index < 0) return
    setInput(messages[index].content)
    removeConversationImages(messages.slice(index))
    setStore((current) => ({
      ...current,
      conversations: current.conversations.map((item) => item.id === current.activeId
        ? { ...item, messages: item.messages.slice(0, index), updatedAt: Date.now() }
        : item),
    }))
  }

  const newChat = () => {
    if (!messages.length) return
    stopMediaForConversationChange()
    activeStream.current?.abort()
    streamGeneration.current += 1
    setStreaming(false)
    const conversation = createConversation(group, model)
    setStore((current) => ({ ...current, activeId: conversation.id, conversations: [...current.conversations, conversation] }))
    setInput('')
    setImagePath('')
    setSettingsOpen(false)
    setHistoryOpen(false)
    setMoreActionsOpen(false)
  }

  const selectChat = (id: string) => {
    if (id === store.activeId) { setHistoryOpen(false); return }
    stopMediaForConversationChange()
    activeStream.current?.abort()
    streamGeneration.current += 1
    setStreaming(false)
    setStore((current) => ({ ...current, activeId: id }))
    setInput('')
    setImagePath('')
    setHistoryOpen(false)
    setSettingsOpen(false)
    setMoreActionsOpen(false)
  }

  const deleteChat = async (id: string) => {
    const result = await Taro.showModal({ title: t('playground.deleteChat'), content: t('playground.deleteChatConfirm') })
    if (!result.confirm) return
    const removed = store.conversations.find((item) => item.id === id)
    if (removed) removeConversationImages(removed.messages)
    if (id === store.activeId) {
      stopMediaForConversationChange()
      activeStream.current?.abort()
      streamGeneration.current += 1
      setStreaming(false)
      setInput('')
      setImagePath('')
    }
    setStore((current) => {
      const conversations = current.conversations.filter((item) => item.id !== id)
      if (!conversations.length) conversations.push(createConversation(group, model))
      return {
        ...current,
        activeId: current.activeId === id ? conversations[0].id : current.activeId,
        conversations,
      }
    })
  }

  if (!signedIn) {
    return (
      <PageShell title={t('playground.title')} description={t('playground.description')}>
        <Text className='mobile-empty'>{t('playground.loginRequired')}</Text>
      </PageShell>
    )
  }

  return (
    <View className='playground-page'>
      <View className='playground-topbar'>
        <Button className='playground-model-trigger' onClick={() => { setHistoryOpen(false); setSettingsOpen((open) => !open) }}>
          <View className='playground-model-trigger__inner'>
            <Text className='playground-model-trigger__name'>{model || t('playground.model')}</Text>
            <View className='playground-model-trigger__chevron' />
          </View>
        </Button>
        <Button className='playground-history-trigger' onClick={() => { setSettingsOpen(false); setHistoryOpen((open) => !open) }}>
          {t('playground.history')}
        </Button>
        <Button className='playground-new-chat' disabled={messages.length === 0} onClick={newChat}>
          {t('playground.newChat')}
        </Button>
      </View>
      {historyOpen ? (
        <View className='playground-history'>
          <View className='playground-history__header'>
            <Text className='playground-history__heading'>{t('playground.history')}</Text>
            <Text className='playground-history__count'>{historyConversations.length}</Text>
          </View>
          {historyConversations.length ? (
            <ScrollView className='playground-history__scroll' scrollY style={{ height: `${Math.min(historyConversations.length * 112, 448)}rpx` }}>
              {historyConversations.map((conversation) => (
                <View className={`playground-history__row ${conversation.id === store.activeId ? 'playground-history__row--active' : ''}`} key={conversation.id}>
                  <Button className='playground-history__select' onClick={() => selectChat(conversation.id)}>
                    <View className='playground-history__title-line'>
                      <Text className='playground-history__title'>{conversationTitle(conversation) || t('playground.untitled')}</Text>
                      {conversation.id === store.activeId ? <Text className='playground-history__current'>{t('playground.currentChat')}</Text> : null}
                    </View>
                    <Text className='playground-history__date'>{new Date(conversation.updatedAt).toLocaleString(undefined, { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}</Text>
                  </Button>
                  <Button className='playground-history__delete' onClick={() => void deleteChat(conversation.id)}>{t('common.delete')}</Button>
                </View>
              ))}
            </ScrollView>
          ) : <Text className='playground-history__empty'>{t('playground.noHistory')}</Text>}
        </View>
      ) : null}
      {settingsOpen ? (
        <View className='playground-options'>
          <Picker mode='selector' range={groups} value={Math.max(0, groups.indexOf(group))} onChange={(event) => setStore((current) => ({ ...current, conversations: current.conversations.map((item) => item.id === current.activeId ? { ...item, group: groups[Number(event.detail.value)] ?? '', model: '' } : item) }))}>
            <View className='mobile-picker'>{t('playground.group')}: {group || '—'}</View>
          </Picker>
          <Picker mode='selector' range={models} value={Math.max(0, models.indexOf(model))} onChange={(event) => { setStore((current) => ({ ...current, conversations: current.conversations.map((item) => item.id === current.activeId ? { ...item, model: models[Number(event.detail.value)] ?? '' } : item) })); setSettingsOpen(false) }}>
            <View className='mobile-picker'>{t('playground.model')}: {model || '—'}</View>
          </Picker>
        </View>
      ) : null}
      {loadingOptions ? <Text className='mobile-muted'>{t('common.loading')}</Text> : null}
      {errorKey ? <Text className='mobile-error'>{t(errorKey)}</Text> : null}

      <ScrollView
        className='playground-chat'
        key={store.activeId}
        scrollY
        scrollTop={messages.reduce((total, message) => total + message.content.length * 2 + 1000, 0)}
      >
        {messages.length === 0 ? (
          <View className='playground-welcome'>
            <Text className='playground-welcome__title'>{t('playground.empty')}</Text>
            <Text className='playground-welcome__hint'>{t('playground.promptHint')}</Text>
            {(['playground.promptOne', 'playground.promptTwo', 'playground.promptThree'] as const).map((key) => (
              <Button className='playground-welcome__prompt' disabled={!model || !group || loadingOptions} key={key} onClick={() => submitMessage(t(key))}>
                {t(key)}
              </Button>
            ))}
          </View>
        ) : null}
        {messages.map((message) => (
          <View className={`playground-message playground-message--${message.role}`} key={message.id}>
            <Text className='playground-message__role'>
              {message.role === 'user' ? t('playground.you') : t('playground.assistant')}
            </Text>
            {message.role === 'assistant' && message.content
              ? <ChatAnswer content={message.content} />
              : <Text className='playground-message__content'>{message.content || (streaming ? t('playground.streaming') : '')}</Text>}
            {message.imagePath ? <Image className='playground-message__image' src={message.imagePath} mode='aspectFill' /> : null}
            {message.role === 'assistant' && message.content && media?.voice_output_models.includes(model) ? (
              <Button className='playground-message__speak' onClick={() => void toggleSpeech(message)}>
                {t(playingId === message.id ? 'playground.stopSpeech' : 'playground.listen')}
              </Button>
            ) : null}
          </View>
        ))}
        <View id='playground-end' />
      </ScrollView>

      <View className='playground-composer'>
        {imagePath ? <View className='playground-attachment'><Image src={imagePath} mode='aspectFill' /><Button onClick={() => { removeConversationImages([{ content: '', createdAt: 0, id: '', imagePath, role: 'user' }]); setImagePath('') }}>{t('playground.removeImage')}</Button></View> : null}
        <View className='playground-compose-row'>
          <Textarea className='playground-input' value={input} maxlength={8000} placeholder={t('playground.input')} onInput={(event) => setInput(event.detail.value)} />
          {streaming ? (
            <Button className='playground-send playground-send--stop' onClick={() => activeStream.current?.abort()}>{t('playground.stop')}</Button>
          ) : (
            <Button className='playground-send' disabled={(!input.trim() && !imagePath) || !model || !group || loadingOptions || transcribing} onClick={() => submitMessage(input)}>{t('playground.send')}</Button>
          )}
        </View>
        <View className='playground-media-actions'>
          {media?.image_input_models.includes(model) ? <Button disabled={streaming} onClick={() => void chooseImage()}>{t('playground.addImage')}</Button> : null}
          {media?.voice_input_models.includes(model) ? <Button disabled={streaming || transcribing} onClick={toggleRecording}>{t(recording ? 'playground.stopRecording' : 'playground.record')}</Button> : null}
          {transcribing ? <Text>{t('playground.transcribing')}</Text> : null}
        </View>
        {messages.length > 0 ? (
          <View className='playground-compose-actions'>
            <Button className='playground-action' disabled={streaming} onClick={retry}>{t('playground.retry')}</Button>
            <Button className='playground-action' onClick={() => setMoreActionsOpen((open) => !open)}>
              {t(moreActionsOpen ? 'playground.hideActions' : 'playground.moreActions')}
            </Button>
          </View>
        ) : null}
        {moreActionsOpen && messages.length > 0 ? (
          <View className='playground-compose-actions'>
            <Button className='playground-action' disabled={streaming} onClick={editLast}>{t('playground.edit')}</Button>
            <Button className='playground-action' onClick={() => void deleteChat(store.activeId)}>{t('playground.deleteChat')}</Button>
          </View>
        ) : null}
      </View>
    </View>
  )
}
