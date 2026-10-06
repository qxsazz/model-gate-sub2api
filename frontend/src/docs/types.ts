export interface DocLocation {
  category: string
  page: string
}

export interface DocArticle {
  slug: string
  title: string
  description: string
  updatedAt: string
  source: string
  status?: 'available' | 'upcoming'
  collapsedHeadings?: string[]
}

export interface DocGroup {
  slug: string
  title: string
  description: string
  articles: DocArticle[]
}

export interface ResolvedDocument {
  location: DocLocation
  group: DocGroup
  document: DocArticle
}

export interface DocHeading {
  id: string
  text: string
  level: 2 | 3
}

export interface RenderedMarkdown {
  html: string
  headings: DocHeading[]
}
