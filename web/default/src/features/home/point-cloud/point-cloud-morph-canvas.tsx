/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import {
  applyPointCloudDragRotation,
  createPointCloudDragRotation,
  settlePointCloudDragRotation,
} from './point-cloud-interaction'
import {
  POINT_CLOUD_FRAGMENT_SHADER,
  POINT_CLOUD_VERTEX_SHADER,
} from './point-cloud-shaders'
import {
  getRuntimePointCloudSequence,
  type PointCloudFaceState,
  type RuntimePointCloudId,
} from './point-cloud-state'

type PointCloudManifestAsset = {
  id: RuntimePointCloudId
  url: string
  pointCount: number
}

type PointCloudManifest = {
  pointCount: number
  assets: PointCloudManifestAsset[]
}

type PointCloudMorphCanvasProps = {
  faceState: PointCloudFaceState
  className?: string
  variant?: 'panel' | 'background'
  pointSize?: number
  showInstruction?: boolean
  morphSignal?: number
}

type GlProgramInfo = {
  program: WebGLProgram
  attributes: {
    from: number
    to: number
    seed: number
  }
  uniforms: {
    morph: WebGLUniformLocation | null
    time: WebGLUniformLocation | null
    aspect: WebGLUniformLocation | null
    pixelRatio: WebGLUniformLocation | null
    pointSize: WebGLUniformLocation | null
    rotationX: WebGLUniformLocation | null
    rotationY: WebGLUniformLocation | null
    mouse: WebGLUniformLocation | null
    mouseStrength: WebGLUniformLocation | null
    colorFrom: WebGLUniformLocation | null
    colorTo: WebGLUniformLocation | null
  }
}

const MANIFEST_URL = '/pointclouds/manifest.json'
const MORPH_DURATION_MS = 2600
const HOLD_DURATION_MS = 4400
const SHAPE_COLORS: Record<RuntimePointCloudId, [number, number, number]> = {
  'face-closed': [0.72, 0.82, 1],
  'face-open': [0.9, 0.96, 1],
  'black-hole': [0.62, 0.68, 1],
  'lorenz-attractor': [0.72, 1, 0.93],
}

function isInteractiveTarget(target: EventTarget | null): boolean {
  if (!(target instanceof Element)) return false
  return Boolean(
    target.closest(
      'a,button,input,textarea,select,[role="button"],[data-point-cloud-ignore]'
    )
  )
}

function compileShader(
  gl: WebGLRenderingContext,
  type: number,
  source: string
): WebGLShader {
  const shader = gl.createShader(type)
  if (!shader) throw new Error('Unable to create WebGL shader')
  gl.shaderSource(shader, source)
  gl.compileShader(shader)
  if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
    const message = gl.getShaderInfoLog(shader) || 'Unknown shader error'
    gl.deleteShader(shader)
    throw new Error(message)
  }
  return shader
}

function createProgram(gl: WebGLRenderingContext): GlProgramInfo {
  const vertex = compileShader(gl, gl.VERTEX_SHADER, POINT_CLOUD_VERTEX_SHADER)
  const fragment = compileShader(
    gl,
    gl.FRAGMENT_SHADER,
    POINT_CLOUD_FRAGMENT_SHADER
  )
  const program = gl.createProgram()
  if (!program) throw new Error('Unable to create WebGL program')

  gl.attachShader(program, vertex)
  gl.attachShader(program, fragment)
  gl.linkProgram(program)
  gl.deleteShader(vertex)
  gl.deleteShader(fragment)

  if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
    const message = gl.getProgramInfoLog(program) || 'Unknown program error'
    gl.deleteProgram(program)
    throw new Error(message)
  }

  return {
    program,
    attributes: {
      from: gl.getAttribLocation(program, 'aFrom'),
      to: gl.getAttribLocation(program, 'aTo'),
      seed: gl.getAttribLocation(program, 'aSeed'),
    },
    uniforms: {
      morph: gl.getUniformLocation(program, 'uMorph'),
      time: gl.getUniformLocation(program, 'uTime'),
      aspect: gl.getUniformLocation(program, 'uAspect'),
      pixelRatio: gl.getUniformLocation(program, 'uPixelRatio'),
      pointSize: gl.getUniformLocation(program, 'uPointSize'),
      rotationX: gl.getUniformLocation(program, 'uRotationX'),
      rotationY: gl.getUniformLocation(program, 'uRotationY'),
      mouse: gl.getUniformLocation(program, 'uMouse'),
      mouseStrength: gl.getUniformLocation(program, 'uMouseStrength'),
      colorFrom: gl.getUniformLocation(program, 'uColorFrom'),
      colorTo: gl.getUniformLocation(program, 'uColorTo'),
    },
  }
}

function createSeedBuffer(pointCount: number): Float32Array {
  const seeds = new Float32Array(pointCount)
  for (let i = 0; i < pointCount; i += 1) {
    seeds[i] = (((Math.sin(i * 12.9898 + 78.233) * 43758.5453) % 1) + 1) % 1
  }
  return seeds
}

function bindArrayBuffer(
  gl: WebGLRenderingContext,
  location: number,
  buffer: WebGLBuffer,
  size: number,
  data: Float32Array
) {
  gl.bindBuffer(gl.ARRAY_BUFFER, buffer)
  gl.bufferData(gl.ARRAY_BUFFER, data, gl.DYNAMIC_DRAW)
  gl.enableVertexAttribArray(location)
  gl.vertexAttribPointer(location, size, gl.FLOAT, false, 0, 0)
}

async function loadPointCloudAssets(sequence: RuntimePointCloudId[]) {
  const manifestResponse = await fetch(MANIFEST_URL, { cache: 'no-cache' })
  if (!manifestResponse.ok) {
    throw new Error(`Point cloud manifest failed: ${manifestResponse.status}`)
  }
  const manifest = (await manifestResponse.json()) as PointCloudManifest
  const byId = new Map(manifest.assets.map((asset) => [asset.id, asset]))
  const loaded = new Map<RuntimePointCloudId, Float32Array>()

  for (const id of sequence) {
    const asset = byId.get(id)
    if (!asset) throw new Error(`Missing point cloud asset: ${id}`)
    if (asset.pointCount !== manifest.pointCount) {
      throw new Error(`Point cloud count mismatch: ${id}`)
    }
    const response = await fetch(asset.url, { cache: 'force-cache' })
    if (!response.ok) throw new Error(`Point cloud failed: ${id}`)
    const points = new Float32Array(await response.arrayBuffer())
    if (points.length !== manifest.pointCount * 3) {
      throw new Error(`Point cloud byte length mismatch: ${id}`)
    }
    loaded.set(id, points)
  }

  return {
    pointCount: manifest.pointCount,
    assets: loaded,
  }
}

function useReducedMotionPreference() {
  const [reduced, setReduced] = useState(false)

  useEffect(() => {
    const query = window.matchMedia('(prefers-reduced-motion: reduce)')
    const update = () => setReduced(query.matches)
    update()
    query.addEventListener('change', update)
    return () => query.removeEventListener('change', update)
  }, [])

  return reduced
}

export function PointCloudMorphCanvas(props: PointCloudMorphCanvasProps) {
  const { t } = useTranslation()
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const advanceRef = useRef<((now: number) => void) | null>(null)
  const previousMorphSignalRef = useRef(props.morphSignal)
  const reducedMotion = useReducedMotionPreference()
  const [ready, setReady] = useState(false)
  const [failed, setFailed] = useState(false)
  const sequence = useMemo(
    () => getRuntimePointCloudSequence(props.faceState),
    [props.faceState]
  )
  const isBackground = props.variant === 'background'
  const pointSize = props.pointSize ?? (isBackground ? 2.1 : 2.4)

  useEffect(() => {
    if (props.morphSignal == null) return
    if (previousMorphSignalRef.current === props.morphSignal) return
    previousMorphSignalRef.current = props.morphSignal
    if (reducedMotion) return
    advanceRef.current?.(performance.now())
  }, [props.morphSignal, reducedMotion])

  useEffect(() => {
    const canvasElement = canvasRef.current
    if (!canvasElement) return
    const canvas: HTMLCanvasElement = canvasElement

    let disposed = false
    let animationFrame = 0
    const pointer = {
      x: 99,
      y: 99,
      strength: 0,
      targetStrength: 0,
      dragging: false,
      lastX: 0,
      lastY: 0,
      dragDistance: 0,
      rotation: createPointCloudDragRotation(),
    }

    const cleanupCallbacks: Array<() => void> = []

    async function init() {
      try {
        setFailed(false)
        setReady(false)
        const gl = canvas.getContext('webgl', {
          alpha: true,
          antialias: false,
          depth: false,
          powerPreference: 'high-performance',
        })
        if (!gl) throw new Error('WebGL unavailable')

        const loaded = await loadPointCloudAssets(sequence)
        if (disposed) return

        const programInfo = createProgram(gl)
        const fromBuffer = gl.createBuffer()
        const toBuffer = gl.createBuffer()
        const seedBuffer = gl.createBuffer()
        if (!fromBuffer || !toBuffer || !seedBuffer) {
          throw new Error('Unable to create point buffers')
        }
        const seeds = createSeedBuffer(loaded.pointCount)

        let fromIndex = 0
        let toIndex = reducedMotion ? 0 : Math.min(1, sequence.length - 1)
        let morphStart = performance.now()
        let holdStart = morphStart

        const bindTargets = () => {
          const fromId = sequence[fromIndex]
          const toId = sequence[toIndex]
          const from = loaded.assets.get(fromId)
          const to = loaded.assets.get(toId)
          if (!from || !to) throw new Error('Missing active point cloud target')
          bindArrayBuffer(gl, programInfo.attributes.from, fromBuffer, 3, from)
          bindArrayBuffer(gl, programInfo.attributes.to, toBuffer, 3, to)
          bindArrayBuffer(gl, programInfo.attributes.seed, seedBuffer, 1, seeds)
        }

        const advance = (now: number) => {
          fromIndex = toIndex
          toIndex = (toIndex + 1) % sequence.length
          morphStart = now
          holdStart = now + MORPH_DURATION_MS
          bindTargets()
        }
        advanceRef.current = advance

        bindTargets()

        const resize = () => {
          const rect = canvas.getBoundingClientRect()
          const pixelRatio = Math.min(window.devicePixelRatio || 1, 2)
          const width = Math.max(1, Math.floor(rect.width * pixelRatio))
          const height = Math.max(1, Math.floor(rect.height * pixelRatio))
          if (canvas.width !== width || canvas.height !== height) {
            canvas.width = width
            canvas.height = height
          }
          gl.viewport(0, 0, width, height)
          return {
            pixelRatio,
            aspect: rect.width / Math.max(rect.height, 1),
          }
        }

        const onPointerMove = (event: PointerEvent) => {
          const rect = canvas.getBoundingClientRect()
          const aspect = rect.width / Math.max(rect.height, 1)
          pointer.x =
            ((event.clientX - rect.left) / Math.max(rect.width, 1) - 0.5) *
            2 *
            aspect
          pointer.y =
            -((event.clientY - rect.top) / Math.max(rect.height, 1) - 0.5) * 2
          pointer.targetStrength = reducedMotion ? 0.08 : 1

          if (pointer.dragging) {
            const deltaX = event.clientX - pointer.lastX
            const deltaY = event.clientY - pointer.lastY
            applyPointCloudDragRotation(pointer.rotation, { deltaX, deltaY })
            pointer.dragDistance += Math.abs(deltaX) + Math.abs(deltaY)
            pointer.lastX = event.clientX
            pointer.lastY = event.clientY
          }
        }
        const onPointerLeave = () => {
          pointer.targetStrength = 0
        }
        const onPointerDown = (event: PointerEvent) => {
          if (event.button !== 0) return
          if (isBackground && isInteractiveTarget(event.target)) return
          pointer.dragging = true
          pointer.lastX = event.clientX
          pointer.lastY = event.clientY
          pointer.dragDistance = 0
          pointer.targetStrength = reducedMotion ? 0.08 : 1
        }
        const onPointerUp = () => {
          if (!pointer.dragging) return
          pointer.dragging = false
          if (!reducedMotion && pointer.dragDistance < 4) {
            advance(performance.now())
          }
        }
        const onPointerCancel = () => {
          pointer.dragging = false
        }
        const pointerMoveListener: EventListener = (event) =>
          onPointerMove(event as PointerEvent)
        const pointerDownListener: EventListener = (event) =>
          onPointerDown(event as PointerEvent)
        const pointerUpListener: EventListener = () => onPointerUp()
        const pointerCancelListener: EventListener = () => onPointerCancel()

        const pointerTarget: Window | HTMLCanvasElement = isBackground
          ? window
          : canvas
        pointerTarget.addEventListener('pointermove', pointerMoveListener)
        pointerTarget.addEventListener('pointerdown', pointerDownListener)
        window.addEventListener('pointerup', pointerUpListener)
        window.addEventListener('pointercancel', pointerCancelListener)
        window.addEventListener('blur', pointerCancelListener)
        if (!isBackground) {
          canvas.addEventListener('pointerleave', onPointerLeave)
        }
        cleanupCallbacks.push(() => {
          pointerTarget.removeEventListener('pointermove', pointerMoveListener)
          pointerTarget.removeEventListener('pointerdown', pointerDownListener)
          window.removeEventListener('pointerup', pointerUpListener)
          window.removeEventListener('pointercancel', pointerCancelListener)
          window.removeEventListener('blur', pointerCancelListener)
          if (!isBackground) {
            canvas.removeEventListener('pointerleave', onPointerLeave)
          }
        })

        gl.useProgram(programInfo.program)
        gl.enable(gl.BLEND)
        gl.blendFunc(gl.SRC_ALPHA, gl.ONE)
        setReady(true)

        const render = (now: number) => {
          if (disposed) return
          const { pixelRatio, aspect } = resize()
          const progress = reducedMotion
            ? 0
            : Math.min((now - morphStart) / MORPH_DURATION_MS, 1)
          if (
            !reducedMotion &&
            progress >= 1 &&
            now > holdStart + HOLD_DURATION_MS
          ) {
            advance(now)
          }

          pointer.strength += (pointer.targetStrength - pointer.strength) * 0.08
          settlePointCloudDragRotation(pointer.rotation)
          gl.clearColor(0, 0, 0, 0)
          gl.clear(gl.COLOR_BUFFER_BIT)
          gl.uniform1f(programInfo.uniforms.morph, progress)
          gl.uniform1f(programInfo.uniforms.time, now * 0.001)
          gl.uniform1f(programInfo.uniforms.aspect, aspect)
          gl.uniform1f(programInfo.uniforms.pixelRatio, pixelRatio)
          gl.uniform1f(programInfo.uniforms.pointSize, pointSize)
          gl.uniform1f(
            programInfo.uniforms.rotationX,
            pointer.rotation.rotationX
          )
          gl.uniform1f(
            programInfo.uniforms.rotationY,
            pointer.rotation.rotationY
          )
          gl.uniform2f(programInfo.uniforms.mouse, pointer.x, pointer.y)
          gl.uniform1f(programInfo.uniforms.mouseStrength, pointer.strength)

          const fromColor = SHAPE_COLORS[sequence[fromIndex]]
          const toColor = SHAPE_COLORS[sequence[toIndex]]
          gl.uniform3f(programInfo.uniforms.colorFrom, ...fromColor)
          gl.uniform3f(programInfo.uniforms.colorTo, ...toColor)
          gl.drawArrays(gl.POINTS, 0, loaded.pointCount)
          animationFrame = requestAnimationFrame(render)
        }

        animationFrame = requestAnimationFrame(render)
        cleanupCallbacks.push(() => {
          gl.deleteBuffer(fromBuffer)
          gl.deleteBuffer(toBuffer)
          gl.deleteBuffer(seedBuffer)
          gl.deleteProgram(programInfo.program)
        })
      } catch (error) {
        if (!disposed) {
          setFailed(true)
          setReady(false)
          // eslint-disable-next-line no-console
          console.error('Point cloud hero failed:', error)
        }
      }
    }

    init()

    return () => {
      disposed = true
      advanceRef.current = null
      cancelAnimationFrame(animationFrame)
      cleanupCallbacks.forEach((callback) => callback())
    }
  }, [isBackground, pointSize, reducedMotion, sequence])

  return (
    <div
      className={cn(
        isBackground
          ? 'fixed inset-0 min-h-[100dvh] overflow-hidden bg-[#030409] opacity-90'
          : 'relative min-h-[520px] overflow-hidden rounded-[2rem] border border-white/10 bg-[#030409] shadow-[0_40px_120px_rgba(0,0,0,0.45)]',
        props.className
      )}
    >
      <canvas
        ref={canvasRef}
        aria-hidden='true'
        className={cn(
          'absolute inset-0 h-full w-full touch-none',
          isBackground ? 'pointer-events-none opacity-90' : 'opacity-95'
        )}
      />
      <div
        aria-hidden='true'
        className={cn(
          'pointer-events-none absolute inset-0',
          isBackground
            ? 'bg-[radial-gradient(circle_at_62%_46%,rgba(160,180,255,0.16),transparent_38%),linear-gradient(90deg,rgba(3,4,9,0.72),rgba(3,4,9,0.18)_42%,rgba(3,4,9,0.78))]'
            : 'bg-[radial-gradient(circle_at_65%_45%,rgba(160,180,255,0.18),transparent_30%),linear-gradient(90deg,rgba(3,4,9,0.1),rgba(3,4,9,0.55))]'
        )}
      />
      <div
        aria-hidden='true'
        className={cn(
          'pointer-events-none absolute inset-0 bg-[repeating-linear-gradient(0deg,rgba(255,255,255,0.026)_0_1px,transparent_1px_10px)]',
          isBackground ? 'opacity-25' : 'opacity-50'
        )}
      />
      {(!ready || failed) && (
        <div className='absolute inset-0 flex items-center justify-center bg-[#030409] text-center'>
          <div className='max-w-xs px-6'>
            <div className='mx-auto mb-4 h-24 w-24 rounded-full border border-white/15 bg-[radial-gradient(circle,rgba(220,230,255,0.28),transparent_62%)]' />
            <p className='text-sm font-medium text-white/78'>
              {failed
                ? t('Point cloud system paused')
                : t('Loading point cloud')}
            </p>
            <p className='mt-2 text-xs leading-relaxed text-white/48'>
              {t(
                'The gateway remains available while the visual layer initializes.'
              )}
            </p>
          </div>
        </div>
      )}
      {props.showInstruction && ready && !failed && (
        <div className='pointer-events-none absolute right-6 bottom-6 hidden max-w-[260px] rounded-2xl border border-white/10 bg-[#030409]/55 p-4 text-xs leading-relaxed text-white/48 backdrop-blur-xl lg:block'>
          {t(
            'Move the pointer through the cloud. Hold and drag to rotate the field.'
          )}
        </div>
      )}
    </div>
  )
}
