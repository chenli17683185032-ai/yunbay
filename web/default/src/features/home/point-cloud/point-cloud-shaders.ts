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
export const POINT_CLOUD_VERTEX_SHADER = `
precision mediump float;

attribute vec3 aFrom;
attribute vec3 aTo;
attribute float aSeed;

uniform float uMorph;
uniform float uTime;
uniform float uAspect;
uniform float uPixelRatio;
uniform float uPointSize;
uniform float uRotationX;
uniform float uRotationY;
uniform vec2 uMouse;
uniform float uMouseStrength;
uniform vec3 uColorFrom;
uniform vec3 uColorTo;

varying vec3 vColor;
varying float vAlpha;

mat3 rotateY(float angle) {
  float c = cos(angle);
  float s = sin(angle);
  return mat3(c, 0.0, -s, 0.0, 1.0, 0.0, s, 0.0, c);
}

mat3 rotateX(float angle) {
  float c = cos(angle);
  float s = sin(angle);
  return mat3(1.0, 0.0, 0.0, 0.0, c, -s, 0.0, s, c);
}

void main() {
  float eased = uMorph * uMorph * (3.0 - 2.0 * uMorph);
  vec3 p = mix(aFrom, aTo, eased);
  p = rotateY(uTime * 0.035 + uRotationY) * rotateX(sin(uTime * 0.12) * 0.06 + uRotationX) * p;

  vec2 mouseDelta = vec2(p.x - uMouse.x, p.y - uMouse.y);
  float mouseDistance = length(mouseDelta);
  float influence = exp(-mouseDistance * mouseDistance * 5.8) * uMouseStrength;
  vec2 direction = normalize(mouseDelta + vec2(0.0001));
  p.xy += direction * influence * 0.22;
  p.z += sin(aSeed * 18.7 + uTime * 3.0) * influence * 0.18;

  float cameraDistance = 3.15;
  float perspective = 2.25 / max(0.8, cameraDistance - p.z);
  vec2 projected = vec2(p.x * perspective / max(uAspect, 0.001), p.y * perspective);

  gl_Position = vec4(projected, p.z * 0.18, 1.0);
  gl_PointSize = clamp(uPointSize * perspective * (1.0 + influence * 1.6), 1.0, 8.0) * uPixelRatio;

  float depth = clamp((p.z + 1.4) / 2.8, 0.0, 1.0);
  vec3 spectral = mix(uColorFrom, uColorTo, eased);
  vColor = mix(spectral * 0.62, spectral, depth);
  vAlpha = 0.48 + depth * 0.48 + influence * 0.2;
}
`

export const POINT_CLOUD_FRAGMENT_SHADER = `
precision mediump float;

varying vec3 vColor;
varying float vAlpha;

void main() {
  vec2 uv = gl_PointCoord - vec2(0.5);
  float distanceToCenter = length(uv);
  float alpha = smoothstep(0.5, 0.05, distanceToCenter) * vAlpha;
  gl_FragColor = vec4(vColor, alpha);
}
`
