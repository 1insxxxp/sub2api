// GLSL for the homepage relay scene. Output is linear HDR; tone mapping happens in post.

export const coreVertex = /* glsl */ `
varying vec3 vNormal;
varying vec3 vView;
varying vec3 vPos;
void main() {
  vPos = position;
  vec4 mv = modelViewMatrix * vec4(position, 1.0);
  vNormal = normalize(normalMatrix * normal);
  vView = normalize(-mv.xyz);
  gl_Position = projectionMatrix * mv;
}
`

// Plasma core: layered sine field, scanning energy bands and a fresnel rim.
export const coreFragment = /* glsl */ `
uniform float uTime;
uniform vec3 uColorA;
uniform vec3 uColorB;
varying vec3 vNormal;
varying vec3 vView;
varying vec3 vPos;
float field(vec3 p) {
  return sin(p.x * 3.1 + uTime * 0.9) * sin(p.y * 3.7 - uTime * 1.2) * sin(p.z * 2.6 + uTime * 0.6);
}
void main() {
  float n = field(vPos * 1.6) * 0.5 + 0.5;
  float bands = smoothstep(0.6, 1.0, sin((vPos.y + n * 0.6) * 14.0 - uTime * 2.2) * 0.5 + 0.5);
  float fresnel = pow(1.0 - max(dot(vNormal, vView), 0.0), 2.4);
  vec3 color = mix(uColorA, uColorB, n);
  color += uColorB * bands * 0.55;
  color += vec3(0.55, 0.85, 1.0) * fresnel * 0.9;
  gl_FragColor = vec4(color * 0.85, 1.0);
}
`

export const uvVertex = /* glsl */ `
varying vec2 vUv;
void main() {
  vUv = uv;
  gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
}
`
// Light beam travelling along a tube: uv.x runs along the curve.
export const beamFragment = /* glsl */ `
uniform float uTime;
uniform float uSpeed;
uniform float uOffset;
uniform float uDirection;
uniform vec3 uColor;
varying vec2 vUv;
void main() {
  float x = uDirection > 0.0 ? vUv.x : 1.0 - vUv.x;
  float head = fract(uTime * uSpeed + uOffset);
  float d = head - x;
  float trail = d >= 0.0 ? exp(-d * 9.0) : exp(d * 60.0);
  float base = 0.08;
  float fade = smoothstep(0.0, 0.06, vUv.x) * smoothstep(1.0, 0.94, vUv.x);
  float a = (base + trail * 1.4) * fade;
  gl_FragColor = vec4(uColor * (0.6 + trail * 3.0), a);
}
`

// Volumetric-ish halo: radial falloff on a camera-facing plane.
export const haloFragment = /* glsl */ `
uniform vec3 uColor;
uniform float uIntensity;
varying vec2 vUv;
void main() {
  float d = length(vUv - 0.5) * 2.0;
  float glow = pow(max(1.0 - d, 0.0), 2.6);
  gl_FragColor = vec4(uColor * glow * uIntensity, glow * uIntensity);
}
`

export const dustVertex = /* glsl */ `
attribute float aScale;
attribute float aPhase;
uniform float uTime;
uniform float uPixelRatio;
uniform float uSize;
varying float vTwinkle;
void main() {
  vec3 p = position;
  p.y += sin(uTime * 0.25 + aPhase) * 0.25;
  p.x += cos(uTime * 0.18 + aPhase * 1.7) * 0.18;
  vec4 mv = modelViewMatrix * vec4(p, 1.0);
  vTwinkle = 0.45 + 0.55 * sin(uTime * 1.6 + aPhase * 6.0);
  gl_PointSize = uSize * aScale * uPixelRatio * (8.0 / -mv.z);
  gl_Position = projectionMatrix * mv;
}
`

export const dustFragment = /* glsl */ `
uniform vec3 uColor;
uniform float uOpacity;
varying float vTwinkle;
void main() {
  float d = length(gl_PointCoord - 0.5);
  float a = smoothstep(0.5, 0.0, d);
  a *= a;
  gl_FragColor = vec4(uColor * (1.0 + vTwinkle), a * vTwinkle * uOpacity);
}
`
