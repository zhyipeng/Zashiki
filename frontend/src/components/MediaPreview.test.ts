import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import type { FilePreview } from '../../bindings/zashiki/internal/filemanager'
import { encodeAssetPath, setMediaStreamBase } from './assetUrl'
import MediaPreview from './MediaPreview.vue'

const STREAM_BASE = 'http://127.0.0.1:53101/media/test-token'
setMediaStreamBase(STREAM_BASE)

function preview(overrides: Partial<FilePreview>): FilePreview {
  return {
    name: 'clip.mp4',
    path: '/media/clip.mp4',
    kind: 'video',
    mimeType: 'video/mp4',
    size: 0,
    version: '',
    content: '',
    dataUrl: '',
    truncated: false,
    message: '',
    ...overrides,
  }
}

describe('MediaPreview', () => {
  it('renders a native video element for video files', () => {
    const wrapper = mount(MediaPreview, { props: { preview: preview({}) } })
    const video = wrapper.find('video')
    expect(video.exists()).toBe(true)
    expect(video.attributes('src')).toBe(`${STREAM_BASE}/${encodeAssetPath('/media/clip.mp4')}`)
    expect(video.attributes('controls')).toBeDefined()
    expect(video.attributes('autoplay')).toBeDefined()
    expect(wrapper.find('audio').exists()).toBe(false)
  })

  it('renders a native audio element for audio files', () => {
    const wrapper = mount(MediaPreview, {
      props: {
        preview: preview({ name: 'song.mp3', path: '/media/song.mp3', kind: 'audio', mimeType: 'audio/mpeg' }),
      },
    })
    expect(wrapper.find('audio').exists()).toBe(true)
    expect(wrapper.find('audio').attributes('src')).toBe(`${STREAM_BASE}/${encodeAssetPath('/media/song.mp3')}`)
    expect(wrapper.find('audio').attributes('autoplay')).toBeDefined()
  })

  it('swaps the media element when another file is previewed', async () => {
    const wrapper = mount(MediaPreview, { props: { preview: preview({}) } })
    await wrapper.setProps({ preview: preview({ name: 'other.mp4', path: '/media/other.mp4' }) })
    expect(wrapper.find('video').attributes('src')).toBe(`${STREAM_BASE}/${encodeAssetPath('/media/other.mp4')}`)
  })

  it('falls back to a friendly message when the codec is unsupported', async () => {
    const wrapper = mount(MediaPreview, {
      props: { preview: preview({ name: 'movie.mkv', mimeType: 'video/x-matroska' }) },
    })
    await wrapper.find('video').trigger('error')
    expect(wrapper.find('video').exists()).toBe(false)
    expect(wrapper.text()).toContain('浏览器不支持此媒体编码格式')
  })

  it('explains the service is unavailable when no stream base is configured', () => {
    setMediaStreamBase('')
    try {
      const wrapper = mount(MediaPreview, { props: { preview: preview({}) } })
      expect(wrapper.find('video').exists()).toBe(false)
      expect(wrapper.text()).toContain('媒体预览服务不可用')
    } finally {
      setMediaStreamBase(STREAM_BASE)
    }
  })
})
