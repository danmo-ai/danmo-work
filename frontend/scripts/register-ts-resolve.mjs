/** Register resolve hook so Node tests can import @/ and extensionless .ts. */
import { register } from 'node:module'

register('./ts-resolve-hook.mjs', import.meta.url)
