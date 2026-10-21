# Curated image prompt library

MODEL-GATE bundles 36 selected, text-to-image examples: 12 product and food photographs, 12 portraits and landscapes, and 12 posters and illustrations. Chinese titles, descriptions and tags are curated locally. The generation text remains the source prompt, except that two CC0 food examples are adapted to match their upstream preview compositions (exploded cake layers and strawberry chocolate cake), model-specific flags are removed, and a stray object-replacement character is removed where present. No image-model choice is forced.

Sources are pinned rather than fetched from an unreviewed repository head at runtime:

| Source | Commit | Reused content | License |
| --- | --- | --- | --- |
| [JeremyGDM / Pixonara](https://github.com/JeremyGDM/awesome-ai-product-photography-prompts) | `6815dab9c17ce20c5c554df4ecf28336fd0aef0a` | 12 prompts (two adapted) and upstream example images | CC0-1.0 |
| [Awesome GPT Image 2 Prompts Contributors](https://github.com/Pixmind-io/awesome-gpt-image-2-prompts) | `4ac21d49915fffd5a5be56f2d5b674b6dda34568` | 24 prompts and upstream example images | MIT |

Full upstream license notices are shipped in `web/public/prompt-library/v1/licenses/`. Each prompt links to its exact GitHub source section and retains author attribution. `manifest.json` records original file paths, upstream credit links, image hashes, prompt hashes and modifications. Example images are resized to at most 960 pixels per side and encoded as WebP; all 36 covers together are about 2.4 MB. Preview images are visual examples, never automatically supplied as reference-image inputs.

The three bundled JSON catalogs and covers are served from the website's own `/canvas-app/prompt-library/v1/` path. They remain usable when GitHub is unavailable. Refreshing a bundled source rereads the curated website catalog; upstream changes require a reviewed collection update. Custom sources keep their existing remote-refresh behavior.

Existing accounts automatically replace the seven retired built-in feeds with the curated collections on hydration. User-created sources and their switches, the refresh schedule, and explicit disabling of all built-ins are preserved. New source IDs isolate the new collections from cached entries belonging to retired feeds. Public example assets are separate from users' private, seven-day media storage.

To update, review examples and their licenses at pinned commits, update the catalog and provenance together, retain author/license notices, and validate every local cover. Bump the catalog path for revisions so saved source signatures detect the update. Check migration from existing accounts, tag/search filters, copy/use in the image workbench, custom-source preservation, and offline loading before release.
