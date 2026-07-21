import { describe, expect, it } from 'vitest'
import { cleanRelativePath, formatBytes, isPotentiallyExecutable, isTerminal, progressPercent, topLevelRoots } from './utils'
import type { LocalFile, Transfer } from './types'

describe('transfer presentation helpers', () => {
  it('formats large values and clamps progress', () => {
    expect(formatBytes(5 * 1024 ** 3)).toBe('5.0 GB')
    expect(progressPercent({ size: 10, progress: 12, status: 'Sending' } as Transfer)).toBe(100)
  })

  it('derives unique top-level roots from browser selections', () => {
    const file = new File(['x'], 'x.txt')
    const values = [
      { file, relativePath: 'project/readme.md' },
      { file, relativePath: 'project/src/main.ts' },
      { file, relativePath: 'notes.txt' },
    ] as LocalFile[]
    expect(topLevelRoots(values)).toEqual(['project', 'notes.txt'])
    expect(cleanRelativePath('../project\\readme.md')).toBe('project/readme.md')
  })

  it('keeps failed transfers in history and identifies executable content', () => {
    expect(isTerminal({ status: 'Failed' } as Transfer)).toBe(true)
    expect(isTerminal({ status: 'Sending' } as Transfer)).toBe(false)
    expect(isPotentiallyExecutable('tools/SETUP.PS1')).toBe(true)
    expect(isPotentiallyExecutable('notes/setup.txt')).toBe(false)
  })
})
