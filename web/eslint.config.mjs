import nextCoreWebVitals from "eslint-config-next/core-web-vitals";
import prettier from "eslint-config-prettier";

const config = [
  ...nextCoreWebVitals,
  prettier,
  { ignores: [".next/**", "node_modules/**", "next-env.d.ts"] },
];

export default config;
