/**
 * Provider Utilities
 */

import ethers from 'ethers'

// Create a fallback provider using a comma delimited
// string of URLs
export const FallbackProvider = (config: string) => {
  const providers = []
  const urls = config.split(',')
  for (const url of urls) {
    const provider = new ethers.providers.StaticJsonRpcProvider(url)
    providers.push(provider)
  }
  return new ethers.providers.FallbackProvider(providers)
}
