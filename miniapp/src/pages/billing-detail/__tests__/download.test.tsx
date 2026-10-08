// @vitest-environment jsdom
import { createElement, type ReactNode } from 'react'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import Taro from '@tarojs/taro'
import { afterEach, expect, test, vi } from 'vitest'
import { getBillingStatement, getBillingArtifactDownloadOptions } from '@/api/usage'
import BillingDetailPage from '../index'

vi.mock('@tarojs/components', () => {
 const element = (tag:string) => (props:Record<string, unknown>) => createElement(tag,{onClick:props.onClick},props.children as ReactNode)
 return { Button:element('button'), Text:element('span'), View:element('div') }
})
vi.mock('@tarojs/taro', () => ({default:{downloadFile:vi.fn(),openDocument:vi.fn()},useRouter:()=>({params:{id:'statement'}})}))
vi.mock('@/api/usage', () => ({getBillingStatement:vi.fn(),getBillingArtifactDownloadOptions:vi.fn()}))
vi.mock('@/hooks/use-page-title', () => ({usePageTitle:vi.fn()}))
vi.mock('react-i18next', () => ({useTranslation:()=>({t:(key:string)=>key})}))
afterEach(()=>{cleanup();vi.clearAllMocks()})

test('customer opens the archived workbook with the explicit Excel document type',async()=>{
 vi.mocked(getBillingStatement).mockResolvedValue({
  artifacts:[{id:1,kind:'xlsx',ordinal:0,rows:0,sha256:'digest',size:100}],customer:{},detail_count:0,events:[],source_warning:'',
  statement:{id:'statement',user_id:42,month:'2026-09',revision:1,status:'issued',confirmed_at:0,created_at:0,due_at:0,end_at:0,issued_at:1,start_at:0},
 } as Awaited<ReturnType<typeof getBillingStatement>>)
 vi.mocked(getBillingArtifactDownloadOptions).mockResolvedValue({url:'https://api.example/files/xlsx/0',header:{Authorization:'Bearer session'}})
 vi.mocked(Taro.downloadFile).mockResolvedValue({statusCode:200,tempFilePath:'/tmp/no-extension'} as Awaited<ReturnType<typeof Taro.downloadFile>>)
 render(<BillingDetailPage />)
 fireEvent.click(await screen.findByRole('button',{name:'usage.openArtifact'}))
 await waitFor(()=>expect(Taro.openDocument).toHaveBeenCalledWith({filePath:'/tmp/no-extension',fileType:'xlsx',showMenu:true}))
 expect(getBillingArtifactDownloadOptions).toHaveBeenCalledWith('statement','xlsx',0)
})
