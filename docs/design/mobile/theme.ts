// Neno design tokens — ported from the Industry design system (styles.css).
export const color = {
  bg: '#f2f2f3',
  surface: '#e9e9ea',
  text: '#1d1f20',
  accent: '#5980a6',
  divider: 'rgba(29,31,32,0.16)',
  cornerMark: 'rgba(29,31,32,0.55)',
  accent100: '#eef6ff', accent200: '#d6ebff', accent300: '#b5d9fd', accent400: '#94bce3',
  accent500: '#749dc4', accent600: '#597ea3', accent700: '#416180', accent800: '#2c455d', accent900: '#1d2d3d',
  neutral100: '#f5f5f8', neutral200: '#e7e7ea', neutral300: '#d4d4d7', neutral400: '#b7b7ba',
  neutral500: '#98989b', neutral600: '#7a7a7d', neutral700: '#5d5d60', neutral800: '#424244', neutral900: '#2b2b2d',
} as const;

export const font = {
  heading: 'BarlowCondensed_600SemiBold',
  body: 'Barlow_400Regular',
  bodyMedium: 'Barlow_500Medium',
  bodyBold: 'Barlow_700Bold',
  mono: 'Menlo',
} as const;

// Sizes used in the design (dp). Multiply by user textScale (1.0–2.0).
export const type = {
  label: 11, caption: 12, small: 13, body: 14, bodyL: 15, ui: 16, reader: 17, verse: 18,
  h6: 19, h5: 20, h4: 22, h3: 26, h2: 30, feed: 30, feedDark: 36, h1: 40, display: 88,
} as const;

export const space = { 1: 3.4, 2: 6.8, 3: 10.2, 4: 13.6, 6: 20.4, 8: 27.2, screen: 18, feed: 20 } as const;
export const radius = 0;
export const hit = { min: 44, rail: 48 } as const;
export const layout = { statusBar: 28, tabBar: 60, frameW: 360, frameH: 760 } as const;

export const shadow = {
  sm: { shadowColor: '#2b2b2d', shadowOpacity: 0.14, shadowRadius: 2, shadowOffset: { width: 0, height: 1 }, elevation: 1 },
  md: { shadowColor: '#2b2b2d', shadowOpacity: 0.16, shadowRadius: 10, shadowOffset: { width: 0, height: 3 }, elevation: 3 },
  lg: { shadowColor: '#2b2b2d', shadowOpacity: 0.22, shadowRadius: 32, shadowOffset: { width: 0, height: 12 }, elevation: 8 },
} as const;

// Blueprint corner mark: 11×11 "+" (1px lines), placed at -6 offset outside each corner.
export const cornerMark = { size: 11, offset: -6, stroke: 1, color: color.cornerMark } as const;

export const icon = { strokeWidth: 1.5, rail: 22, tab: 20, inline: 14 } as const;
