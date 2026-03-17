// @ts-ignore — VitePress supports async config at runtime

async function getLatestVersion(repo: string): Promise<string | null> {
  try {
    const res = await fetch(`https://api.github.com/repos/1broseidon/${repo}/releases/latest`)
    if (!res.ok) return null
    const data = await res.json() as { tag_name: string }
    return data.tag_name ?? null
  } catch {
    return null
  }
}

export default (async () => {
  const version = await getLatestVersion('oneagent')

  return {
    title: 'oneagent',
    description: 'Config-driven multi-agent CLI',
    base: '/oneagent/',
    appearance: false,
    cleanUrls: true,
    head: [
      ['link', { rel: 'preconnect', href: 'https://fonts.googleapis.com' }],
      ['link', { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' }],
      ['link', { href: 'https://fonts.googleapis.com/css2?family=Work+Sans:wght@300;400;700&family=JetBrains+Mono:wght@400;500&display=swap', rel: 'stylesheet' }],
    ],
    themeConfig: {
      version,
      nav: [
        { text: 'Guide', link: '/guide/getting-started' },
        { text: 'Config', link: '/reference/config' },
        { text: 'Library', link: '/reference/library' },
        { text: 'Changelog', link: '/changelog' },
      ],
      sidebar: [
        {
          text: 'Guide',
          items: [
            { text: 'Getting Started', link: '/guide/getting-started' },
            { text: 'Output Formats', link: '/guide/output' },
            { text: 'Portable Threads', link: '/guide/threads' },
            { text: 'Agent-as-Tool', link: '/guide/agent-as-tool' },
          ],
        },
        {
          text: 'Reference',
          items: [
            { text: 'Backend Config', link: '/reference/config' },
            { text: 'Go Library', link: '/reference/library' },
            { text: 'Example App', link: '/reference/example' },
          ],
        },
        {
          text: 'Changelog',
          link: '/changelog',
        },
      ],
      socialLinks: [
        { icon: 'github', link: 'https://github.com/1broseidon/oneagent' },
      ],
      outline: { level: [2, 3] },
    },
  }
})
