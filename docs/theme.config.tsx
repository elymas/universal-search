const config = {
  project: {
    link: "https://github.com/elymas/universal-search",
  },
  docsRepositoryBase:
    "https://github.com/elymas/universal-search/tree/main/docs",
  useNextSeoProps: true,
  head: (
    <>
      <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      <meta
        name="description"
        content="Universal Search - Hybrid AI-powered search engine documentation"
      />
      <link rel="icon" href="/favicon.ico" type="image/ico" />
    </>
  ),
  logo: <span>Universal Search</span>,
  logoLink: "https://universal-search.elymas.com",
  footer: {
    text: "Universal Search Documentation",
    components: {
      Copyright: () => (
        <div style={{ display: "flex", gap: "1rem" }}>
          <span>
            © {new Date().getFullYear()} Universal Search. Apache 2.0 licensed.
          </span>
          <a
            href="https://github.com/elymas/universal-search/blob/main/CHANGELOG.md"
            target="_blank"
            rel="noopener noreferrer"
          >
            Changelog
          </a>
        </div>
      ),
    },
  },
  sidebar: {
    defaultMenuCollapseLevel: 1,
    toggleButton: true,
  },
  toc: {
    float: true,
  },
  search: {
    placeholder: "Search documentation...",
    debounce: 300,
  },
  i18n: [
    {
      locale: "en",
      text: "English",
    },
    {
      locale: "ko",
      text: "한국어",
    },
  ],
  darkMode: true,
  nextThemes: {
    defaultTheme: "system",
  },
  feedback: {
    content: "Question? Give feedback →",
    useLink: () => "https://github.com/elymas/universal-search/issues/new",
  },
  editLink: {
    text: "Edit this page on GitHub →",
  },
  navigation: true,
};

export default config;
