/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
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
        ink: "#0f172a",
        muted: "#64748b",
        line: "#e9edf3",
        page: "#f5f7fa",
      },
      fontFamily: {
        sans: ["Inter", "system-ui", "sans-serif"],
        mono: ["ui-monospace", "SFMono-Regular", "Menlo", "monospace"],
      },
    },
  },
  plugins: [],
};
