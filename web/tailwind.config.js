/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        // Unico brand — swap these to the exact Unico palette when confirmed.
        brand: {
          50: "#eaf1fb",
          100: "#cddef5",
          200: "#9dbdea",
          500: "#1466d6",
          600: "#1157bb",
          700: "#0e4796",
        },
        // Semantic tokens driven by CSS variables so they flip in dark mode
        // (see :root / .dark in index.css). Channels are space-separated RGB.
        ink: "rgb(var(--ink) / <alpha-value>)",
        muted: "rgb(var(--muted) / <alpha-value>)",
        line: "rgb(var(--line) / <alpha-value>)",
        page: "rgb(var(--page) / <alpha-value>)",
        card: "rgb(var(--card) / <alpha-value>)",
      },
      fontFamily: {
        sans: ["Inter", "system-ui", "sans-serif"],
        mono: ["ui-monospace", "SFMono-Regular", "Menlo", "monospace"],
      },
    },
  },
  plugins: [],
};
