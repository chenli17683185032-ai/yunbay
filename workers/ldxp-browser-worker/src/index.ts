import { loadConfigFromEnv } from './config.js'

const config = loadConfigFromEnv()
console.log(`LDXP browser worker ${config.workerId} configured for ${config.backendBaseUrl}`)
