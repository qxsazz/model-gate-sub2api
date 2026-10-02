const icons = {
  tutorial: 'book', clients: 'terminal', platform: 'cube', membership: 'creditCard',
  achievements: 'trophy', tools: 'cog', about: 'shield', troubleshooting: 'questionCircle',
} as const

export const getDocCategoryIcon = (slug: string) => icons[slug as keyof typeof icons] || 'book'
