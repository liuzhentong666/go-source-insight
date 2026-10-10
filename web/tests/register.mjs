import { registerHooks } from 'node:module';
import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith('.') && context.parentURL && !/\.[a-z]+$/i.test(specifier)) {
      const target = new URL(`${specifier}.ts`, context.parentURL);
      if (existsSync(fileURLToPath(target))) return nextResolve(target.href, context);
      const index = new URL(`${specifier}/index.ts`, context.parentURL);
      if (existsSync(fileURLToPath(index))) return nextResolve(index.href, context);
    }
    return nextResolve(specifier, context);
  },
});
