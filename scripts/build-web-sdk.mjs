import { copyFile } from 'node:fs/promises';
import { execFileSync } from 'node:child_process';
execFileSync('npm', ['--prefix', 'sdk/web', 'run', 'build'], { stdio: 'inherit' });
await copyFile('sdk/web/dist/index.js', 'internal/api/assets/feedback.js');
