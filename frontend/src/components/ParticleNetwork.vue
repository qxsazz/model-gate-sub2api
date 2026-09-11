<template>
  <canvas ref="canvasRef" class="particle-canvas" />
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue'

const props = defineProps({
  /** 基础粒子数量，默认150 */
  count: { type: Number, default: 125 },
  /** 粒子上限，超出时删掉最老的粒子腾空间，默认320 */
  maxParticles: { type: Number, default: 260 },
  /** 每次点击产生的粒子数，默认12（更少） */
  explodeCount: { type: Number, default: 8 },
  /** 连线距离阈值(px)，默认110（更短 → 连线更稀疏） */
  connectionDist: { type: Number, default: 110 },
  /** 吸附环内半径(px)，中心留空（更大），默认120 */
  innerRadius: { type: Number, default: 120 },
  /** 吸附环外半径(px)，更靠近内半径 → 环带更窄，默认170 */
  outerRadius: { type: Number, default: 170 },
  /** 被吸附粒子间最小间距(px)，斥力作用距离，默认50（大值=分散更开、更快） */
  separation: { type: Number, default: 50 },
  /** 斥力强度，越大分散越快，默认0.75 */
  repelStrength: { type: Number, default: 0.75 },
  /** 吸附跟随强度(0~1)，默认0.2（更强、跟得更紧） */
  snapStrength: { type: Number, default: 0.02 },
  /** 半径平滑趋向速率，默认0.04。控制太近的粒子被"推出去"的过程速度，越小过程越慢越明显 */
  radiusRate: { type: Number, default: 0.015 },
  /** 恢复正常速度系数，默认0.012 */
  resumeRate: { type: Number, default: 0.012 },
  /** 粒子速度倍率，默认1 */
  speedMultiplier: { type: Number, default: 1 },
  /** 背景颜色，默认纯黑 */
  bgColor: { type: String, default: '#000000' },
  /** 粒子颜色模式：'blue' (默认蓝色) 或 'cream' (米黄色) */
  particleColor: { type: String, default: 'blue' },
  /** 是否显示鼠标连线（粒子→鼠标的放射线），默认false */
  showMouseLine: { type: Boolean, default: false },
  /** 是否穿屏（从对侧出来），默认true */
  wrapEdges: { type: Boolean, default: true },
})

const emit = defineEmits(['click', 'particle-count', 'animation-complete'])

const canvasRef = ref(null)

let W = 0, H = 0
let mouse = { x: -9999, y: -9999 }
let particles = []
let rafId = null
let animationState = 'gathering' // gathering -> exploding -> normal
let animationProgress = 0
const centerX = () => W / 2
const centerY = () => H / 2

// 基础漂浮速度：缓慢但肉眼可见
const BASE_SPEED = 0.06
// —— 纯力吸附模型：只施加「朝鼠标的吸力」，不锁定位置，粒子靠惯性自由、不黏手 ——
const ATTRACT = 0.05        // 朝鼠标的吸力（加速度）
const PUSH = 0.12           // 内半径内的外推力（加速度），太近的粒子被缓缓顶开（强于吸力，确保中心留空）
const BOUND = 0.035         // 外边界软拉回力：防止被斥力推出吸附区
const DAMP = 0.93           // 速度阻尼，避免吸力无限累加
const DRIFT_FORCE = 0.02    // 被吸附粒子的微扰，保持轻微"呼吸"感
// 鼠标移动速度阈值(px/帧)：超过此速度时，吸附中的粒子会被"甩开"，即快速移动可脱离吸附
const ESCAPE_SPEED = 7
// 鼠标移动速度追踪（用于快速移动时甩开吸附粒子）
let mouseSpeed = 0
let lastMx = -9999, lastMy = -9999
let time = 0 // 全局时间，用于粒子闪烁/呼吸

function clamp(v, min, max) {
  return Math.max(min, Math.min(max, v))
}

class Particle {
  constructor(w, h, x, y, isExplosion = false, speedMult = 1, innerRadius = 60, outerRadius = 200) {
    if (isExplosion) {
      this.x = x
      this.y = y
      const angle = Math.random() * Math.PI * 2
      const speed = (Math.random() * 3.5 + 2.5) * speedMult
      this.vx = Math.cos(angle) * speed
      this.vy = Math.sin(angle) * speed
      this.radius = Math.random() * 2 + 1.2
      this.hue = 205 + Math.random() * 25
      this.isExplosion = true
    } else {
      this.x = Math.random() * w
      this.y = Math.random() * h
      const angle = Math.random() * Math.PI * 2
      const speed = (Math.random() * BASE_SPEED + BASE_SPEED * 0.5) * speedMult
      this.vx = Math.cos(angle) * speed
      this.vy = Math.sin(angle) * speed
      this.radius = Math.random() * 1.5 + 0.8
      this.hue = 210 + Math.random() * 28
      this.isExplosion = false
    }
    this.initVx = this.vx
    this.initVy = this.vy
    this.snapped = false
    this.driftAmt = 0.5 + Math.random() * 1.5
    this.phase = Math.random() * Math.PI * 2
    this.twinkle = 0.006 + Math.random() * 0.012
    this.targetAngle = Math.random() * Math.PI * 2
    this.orbitRadius = 0
    this.color = '#4da3ff' // 初始颜色，会在绘制时动态更新
  }

  // 动态计算颜色，根据当前 colorMode
  getColor(colorMode) {
    if (colorMode === 'cream') {
      // 米黄色：HSL(40-55, 70-90%, 50-70%)
      const hue = 40 + (this.hue - 210) / 28 * 15  // 映射到米黄色范围
      const saturation = 52
      const lightness = this.isExplosion ? 56 : 47
      return `hsl(${hue}, ${saturation}%, ${lightness}%)`
    } else {
      // 蓝色：原有逻辑
      const saturation = this.isExplosion ? 85 + Math.random() * 15 : 75 + Math.random() * 20
      const lightness = this.isExplosion ? 70 + Math.random() * 15 : 45 + Math.random() * 18
      return `hsl(${this.hue}, ${saturation}%, ${lightness}%)`
    }
  }

  wrap(w, h, enabled) {
    if (!enabled) return
    if (this.x < 0) this.x = w
    if (this.x > w) this.x = 0
    if (this.y < 0) this.y = h
    if (this.y > h) this.y = 0
  }

  update(cfg) {
    const { mousePos, innerRadius, outerRadius, resumeRate, w, h, wrapEnabled, animState, animProgress } = cfg

    // Opening animation state
    if (animState === 'gathering') {
      // Gather to sphere
      const cx = w / 2
      const cy = h / 2
      const targetRadius = Math.min(w, h) * 0.15
      const targetX = cx + Math.cos(this.targetAngle) * targetRadius
      const targetY = cy + Math.sin(this.targetAngle) * targetRadius

      const dx = targetX - this.x
      const dy = targetY - this.y
      const dist = Math.hypot(dx, dy)

      if (dist > 2) {
        const speed = 0.08 * (1 + animProgress)
        this.x += dx * speed
        this.y += dy * speed
      }
      return
    } else if (animState === 'exploding') {
      // Explode outward
      const cx = w / 2
      const cy = h / 2
      const dx = this.x - cx
      const dy = this.y - cy
      const angle = Math.atan2(dy, dx)
      const speed = 8 * animProgress
      this.vx = Math.cos(angle) * speed
      this.vy = Math.sin(angle) * speed
      this.x += this.vx
      this.y += this.vy
      this.wrap(w, h, wrapEnabled)
      return
    }

    // 1. 爆炸火花：高速飞出后逐渐减速，减速完转为普通漂浮粒子（保留在页面上）
    if (this.isExplosion) {
      const sp = Math.hypot(this.vx, this.vy)
      if (sp > 0.25) {
        this.vx *= 0.972
        this.vy *= 0.972
        this.x += this.vx
        this.y += this.vy
        this.wrap(w, h, wrapEnabled)
        return
      } else {
        // 减速完成，融入背景漂浮
        this.isExplosion = false
        const ang = Math.atan2(this.vy, this.vx)
        const ns = (Math.random() * BASE_SPEED + BASE_SPEED * 0.5) * (cfg.speedMult || 1)
        this.vx = Math.cos(ang) * ns
        this.vy = Math.sin(ang) * ns
        this.initVx = this.vx
        this.initVy = this.vy
      }
    }

    // 2. 普通粒子：纯力吸附（只吸不黏）或 自由漂浮
    const dx = mousePos.x - this.x
    const dy = mousePos.y - this.y
    const dist = Math.hypot(dx, dy)

    if (mousePos.x > -999 && dist < outerRadius * 1.2 && mouseSpeed < ESCAPE_SPEED) {
      this.snapped = true
      const nx = dx / (dist || 1)
      const ny = dy / (dist || 1)
      if (dist < innerRadius) {
        // 太靠近中心：向外推的力（过程式——因为是力，会自然缓缓顶开，而非瞬移）
        const push = (innerRadius - dist) / innerRadius * PUSH
        this.vx -= nx * push
        this.vy -= ny * push
      } else {
        // 正常吸附：朝鼠标方向施加吸力（只是力，不锁定位置）
        this.vx += nx * ATTRACT
        this.vy += ny * ATTRACT
      }
      // 外边界软拉回：被斥力推开时也不会掉出吸附区，保持吸附状态
      if (dist > outerRadius * 0.9) {
        const t = Math.min(1, (dist - outerRadius * 0.9) / (outerRadius * 0.3))
        this.vx -= nx * BOUND * t
        this.vy -= ny * BOUND * t
      }
      // 轻微微扰（个性幅度）：保持"呼吸"感、不死板、不过分旋转
      this.vx += (Math.random() - 0.5) * DRIFT_FORCE * this.driftAmt
      this.vy += (Math.random() - 0.5) * DRIFT_FORCE * this.driftAmt
      // 速度阻尼：吸力不会无限累加，达到平衡速度 → 被吸住但在自由漂移，不黏鼠标
      this.vx *= DAMP
      this.vy *= DAMP
    } else {
      // 超出范围 / 鼠标快速移动：慢慢恢复原本的漂浮速度（自然甩开）
      this.vx += (this.initVx - this.vx) * resumeRate
      this.vy += (this.initVy - this.vy) * resumeRate
      this.snapped = false
    }

    this.x += this.vx
    this.y += this.vy
    this.wrap(w, h, wrapEnabled)
  }

  draw(ctx, colorMode) {
    // 闪烁：每个粒子独立相位，轻微明暗呼吸
    const a = colorMode === 'cream'
      ? 0.78 + 0.22 * Math.sin(time * this.twinkle + this.phase)
      : 0.55 + 0.45 * Math.sin(time * this.twinkle + this.phase)
    // 外发光晕（叠加模式下自然形成辉光）
    ctx.globalAlpha = 0.18 * a
    ctx.beginPath()
    ctx.arc(this.x, this.y, this.radius * 3.2, 0, Math.PI * 2)
    ctx.fillStyle = this.color
    ctx.fill()
    // 实心核心
    ctx.globalAlpha = a
    ctx.beginPath()
    ctx.arc(this.x, this.y, this.radius, 0, Math.PI * 2)
    ctx.fillStyle = this.color
    ctx.fill()
    ctx.globalAlpha = 1
  }
}

function init() {
  const canvas = canvasRef.value
  if (!canvas) return
  const dpr = window.devicePixelRatio || 1
  W = canvas.parentElement?.clientWidth ?? window.innerWidth
  H = canvas.parentElement?.clientHeight ?? window.innerHeight
  canvas.width = W * dpr
  canvas.height = H * dpr
  canvas.style.width = W + 'px'
  canvas.style.height = H + 'px'
  const ctx = canvas.getContext('2d')
  ctx.scale(dpr, dpr)

  particles = []
  for (let i = 0; i < props.count; i++) {
    particles.push(new Particle(W, H, 0, 0, false, props.speedMultiplier, props.innerRadius, props.outerRadius))
  }
  emit('particle-count', particles.length)
}

// 被吸附粒子之间的斥力：沿环带周向互相推开，均匀分散成多边形顶点
function applyRepulsion() {
  const snapped = particles.filter(p => p.snapped)
  for (let i = 0; i < snapped.length; i++) {
    for (let j = i + 1; j < snapped.length; j++) {
      const a = snapped[i]
      const b = snapped[j]
      const dx = a.x - b.x
      const dy = a.y - b.y
      const d = Math.hypot(dx, dy)
      if (d < props.separation && d > 0.01) {
        const force = (props.separation - d) / props.separation * props.repelStrength
        const nx = dx / d
        const ny = dy / d
        a.x += nx * force
        a.y += ny * force
        b.x -= nx * force
        b.y -= ny * force
      }
    }
  }
  // 注：不再写死位置。被吸附粒子若被斥力推到外边界附近，会在 update() 内由 BOUND 软拉回，保持吸附状态。
}

function explode(x, y) {
  // 到达上限时删掉最老的粒子腾出空间（火花现在会永久保留）
  while (particles.length + props.explodeCount > props.maxParticles) {
    particles.shift()
  }
  for (let i = 0; i < props.explodeCount; i++) {
    particles.push(new Particle(W, H, x, y, true, props.speedMultiplier, props.innerRadius, props.outerRadius))
  }
}

function drawConnections(ctx) {
  const { connectionDist, outerRadius, showMouseLine } = props

  for (let i = 0; i < particles.length; i++) {
    for (let j = i + 1; j < particles.length; j++) {
      const dx = particles[i].x - particles[j].x
      const dy = particles[i].y - particles[j].y
      const dist = Math.hypot(dx, dy)
      if (dist < connectionDist) {
        const alpha = (1 - dist / connectionDist) * 0.28
        ctx.beginPath()
        ctx.moveTo(particles[i].x, particles[i].y)
        ctx.lineTo(particles[j].x, particles[j].y)
        ctx.strokeStyle = props.particleColor === 'cream'
          ? `rgba(157, 125, 62, ${alpha * 1.6})`
          : `rgba(80, 140, 235, ${alpha})`
        ctx.lineWidth = 0.5
        ctx.stroke()
      }
    }

    if (showMouseLine && particles[i].snapped && mouse.x > -999) {
      const dx = mouse.x - particles[i].x
      const dy = mouse.y - particles[i].y
      const dist = Math.hypot(dx, dy)
      if (dist < outerRadius) {
        const alpha = (1 - dist / outerRadius) * 0.7
        ctx.beginPath()
        ctx.moveTo(particles[i].x, particles[i].y)
        ctx.lineTo(mouse.x, mouse.y)
        ctx.strokeStyle = `rgba(180, 140, 255, ${alpha})`
        ctx.lineWidth = 0.8
        ctx.stroke()
      }
    }
  }
}

function animate() {
  time += 1
  const canvas = canvasRef.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  const dpr = window.devicePixelRatio || 1

  ctx.save()
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)

  ctx.fillStyle = props.bgColor
  ctx.fillRect(0, 0, W, H)

  // Update animation state
  if (animationState === 'gathering') {
    animationProgress += 0.02
    if (animationProgress >= 1) {
      animationState = 'exploding'
      animationProgress = 0
    }
  } else if (animationState === 'exploding') {
    animationProgress += 0.06
    if (animationProgress >= 1) {
      animationState = 'normal'
      animationProgress = 0
      emit('animation-complete')
    }
  }

  const cfg = {
    mousePos: mouse,
    innerRadius: props.innerRadius,
    outerRadius: props.outerRadius,
    snapStrength: props.snapStrength,
    resumeRate: props.resumeRate,
    radiusRate: props.radiusRate,
    w: W,
    h: H,
    wrapEnabled: props.wrapEdges,
    speedMult: props.speedMultiplier,
    animState: animationState,
    animProgress: animationProgress,
  }

  // 计算鼠标每帧位移速度，供「快速移动甩开吸附」使用
  if (mouse.x > -999 && lastMx > -999) {
    mouseSpeed = Math.hypot(mouse.x - lastMx, mouse.y - lastMy)
  } else {
    mouseSpeed = 0
  }
  lastMx = mouse.x
  lastMy = mouse.y

  particles.forEach(p => p.update(cfg))
  if (animationState === 'normal') {
    applyRepulsion()
  }
  // 叠加混合：粒子与连线重叠处自然增亮，呈现霓虹科技感
  ctx.globalCompositeOperation = props.particleColor === 'cream' ? 'source-over' : 'lighter'
  particles.forEach(p => {
    p.color = p.getColor(props.particleColor)
    p.draw(ctx, props.particleColor)
  })
  if (animationState === 'normal') {
    drawConnections(ctx)
  }
  ctx.globalCompositeOperation = 'source-over'

  ctx.restore()
  rafId = requestAnimationFrame(animate)
}

function onResize() {
  cancelAnimationFrame(rafId)
  init()
  rafId = requestAnimationFrame(animate)
}

onMounted(() => {
  init()
  window.addEventListener('resize', onResize)
  rafId = requestAnimationFrame(animate)

  const canvas = canvasRef.value
  canvas.addEventListener('mousemove', (e) => {
    const rect = canvas.getBoundingClientRect()
    mouse.x = e.clientX - rect.left
    mouse.y = e.clientY - rect.top
  })
  canvas.addEventListener('mouseleave', () => {
    mouse.x = -9999
    mouse.y = -9999
  })
  canvas.addEventListener('click', (e) => {
    const rect = canvas.getBoundingClientRect()
    const x = e.clientX - rect.left
    const y = e.clientY - rect.top
    emit('click', { x, y })
    explode(x, y)
  })
})

onBeforeUnmount(() => {
  cancelAnimationFrame(rafId)
  window.removeEventListener('resize', onResize)
  const canvas = canvasRef.value
  if (canvas) {
    canvas.removeEventListener('mousemove', () => {})
    canvas.removeEventListener('mouseleave', () => {})
    canvas.removeEventListener('click', () => {})
  }
})
</script>

<style scoped>
.particle-canvas {
  display: block;
  width: 100%;
  height: 100%;
}
</style>
