import type { Config } from 'tailwindcss'

const config: Config = {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    container: {
      center: true,
      padding: '1rem',
    },
    extend: {
      colors: {
        // ----- background layers -----
        bg: {
          base: '#0a0d12',
          surface: '#11151c',
          elevated: '#161b24',
          hover: '#1c2230',
        },
        border: {
          subtle: '#1f2532',
          DEFAULT: '#2a3142',
          strong: '#3a4256',
        },
        // ----- text -----
        fg: {
          primary: '#e6e9ef',
          secondary: '#9aa3b2',
          tertiary: '#6b7280',
          disabled: '#4a5160',
        },
        // ----- accent -----
        accent: {
          DEFAULT: '#5b8def',
          hover: '#7aa5ff',
          bg: 'rgba(91, 141, 239, 0.12)',
          border: 'rgba(91, 141, 239, 0.3)',
        },
        // ----- semantic -----
        success: { DEFAULT: '#3ec28f', bg: 'rgba(62, 194, 143, 0.12)' },
        warning: { DEFAULT: '#f5a623', bg: 'rgba(245, 166, 35, 0.12)' },
        danger:  { DEFAULT: '#ef5a6b', bg: 'rgba(239, 90, 107, 0.12)' },
        info:    { DEFAULT: '#6bbcf0' },
        // ----- chart -----
        rx: '#3ec28f',
        tx: '#f5a623',
      },
      fontFamily: {
        sans: ['Inter', 'ui-sans-serif', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'ui-monospace', 'SFMono-Regular', 'Menlo', 'monospace'],
      },
      fontSize: {
        '2xs': ['0.6875rem', '0.875rem'], // 11px
      },
      borderRadius: {
        sm: '6px',
        DEFAULT: '8px',
        md: '10px',
        lg: '14px',
      },
      boxShadow: {
        'sm-dark': '0 1px 2px rgba(0,0,0,0.4)',
        'md-dark': '0 4px 12px rgba(0,0,0,0.3)',
        'lg-dark': '0 8px 24px rgba(0,0,0,0.4)',
      },
      keyframes: {
        'pulse-dot': {
          '50%': { opacity: '0.4' },
        },
        'fade-in': {
          from: { opacity: '0' },
          to: { opacity: '1' },
        },
        'slide-in-right': {
          from: { transform: 'translateX(8px)', opacity: '0' },
          to: { transform: 'translateX(0)', opacity: '1' },
        },
      },
      animation: {
        'pulse-dot': 'pulse-dot 2s infinite',
        'fade-in': 'fade-in 200ms ease-out',
        'slide-in-right': 'slide-in-right 200ms ease-out',
      },
    },
  },
  plugins: [],
}

export default config
