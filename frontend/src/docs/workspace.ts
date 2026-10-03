import { inject, type InjectionKey } from 'vue'
export const documentationPathKey: InjectionKey<string> =
  Symbol('documentation-path')
export const useDocumentationPath = () => inject(documentationPathKey, '/docs')
