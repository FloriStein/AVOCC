import { useCallback, useEffect, useRef, useState } from 'react'

function pickMimeType(): string {
  const candidates = ['video/webm;codecs=vp9', 'video/webm;codecs=vp8', 'video/webm']
  return candidates.find(t => MediaRecorder.isTypeSupported(t)) ?? 'video/webm'
}

function formatDuration(secs: number): string {
  const m = Math.floor(secs / 60).toString().padStart(2, '0')
  const s = (secs % 60).toString().padStart(2, '0')
  return `${m}:${s}`
}

export function useMediaRecorder(streamRef: React.RefObject<MediaStream | null>) {
  const [isRecording, setIsRecording] = useState(false)
  const [duration, setDuration] = useState(0)
  const recorderRef = useRef<MediaRecorder | null>(null)
  const chunksRef = useRef<Blob[]>([])
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const stopInterval = () => {
    if (intervalRef.current) {
      clearInterval(intervalRef.current)
      intervalRef.current = null
    }
  }

  const stop = useCallback(() => {
    if (recorderRef.current && recorderRef.current.state !== 'inactive') {
      recorderRef.current.stop()
    }
    stopInterval()
    setIsRecording(false)
    setDuration(0)
  }, [])

  const start = useCallback(() => {
    const stream = streamRef.current
    if (!stream || isRecording) return

    chunksRef.current = []
    const mimeType = pickMimeType()
    const recorder = new MediaRecorder(stream, { mimeType })
    recorderRef.current = recorder

    recorder.ondataavailable = (e) => {
      if (e.data.size > 0) chunksRef.current.push(e.data)
    }

    recorder.onstop = () => {
      const blob = new Blob(chunksRef.current, { type: mimeType })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      const ts = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)
      a.href = url
      a.download = `avoc-${ts}.webm`
      a.click()
      URL.revokeObjectURL(url)
    }

    recorder.start(1000)
    setIsRecording(true)

    let secs = 0
    intervalRef.current = setInterval(() => setDuration(++secs), 1000)
  }, [streamRef, isRecording])

  useEffect(() => {
    return () => {
      if (recorderRef.current?.state !== 'inactive') recorderRef.current?.stop()
      stopInterval()
    }
  }, [])

  return { isRecording, duration: formatDuration(duration), start, stop }
}
