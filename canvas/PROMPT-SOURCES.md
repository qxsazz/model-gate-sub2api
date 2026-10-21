# Curated image prompt library

MODEL-GATE bundles 72 selected, text-to-image examples: 12 product and food photographs, 12 portraits and landscapes, 12 posters and illustrations, and 36 cinematic character, scene, action and fantasy examples. Chinese titles, descriptions and tags are curated locally. The generation text remains the source prompt, except that two CC0 food examples are adapted to match their upstream preview compositions (exploded cake layers and strawberry chocolate cake), model-specific flags are removed, and a stray object-replacement character is removed where present. No image-model choice is forced.

Sources are pinned rather than fetched from an unreviewed repository head at runtime:

| Source | Commit | Reused content | License |
| --- | --- | --- | --- |
| [JeremyGDM / Pixonara](https://github.com/JeremyGDM/awesome-ai-product-photography-prompts) | `6815dab9c17ce20c5c554df4ecf28336fd0aef0a` | 12 prompts (two adapted) and upstream example images | CC0-1.0 |
| [Awesome GPT Image 2 Prompts Contributors](https://github.com/Pixmind-io/awesome-gpt-image-2-prompts) | `4ac21d49915fffd5a5be56f2d5b674b6dda34568` | 60 prompts and upstream example images | MIT |

Full upstream license notices are shipped in `web/public/prompt-library/v2/licenses/`. Each prompt links to its exact GitHub source section and retains author attribution. `manifest.json` records original file paths, upstream credit links, image hashes, prompt hashes and modifications. Example images are resized to at most 960 pixels per side and encoded as WebP; all 72 covers together are under 6 MB. Preview images are visual examples, never automatically supplied as reference-image inputs.

The six bundled JSON catalogs and covers are served from the website's own `/canvas-app/prompt-library/v2/` path. They remain usable when GitHub is unavailable. Refreshing a bundled source rereads the curated website catalog; upstream changes require a reviewed collection update. Custom sources keep their existing remote-refresh behavior.

Administrator accounts replace retired built-in feeds with the curated collections on hydration, preserving their custom sources, switches, refresh schedule, and explicit disabling of all built-ins. Ordinary accounts always load the six bundled collections; legacy personal-source state remains in its account record but is neither read nor executed. New source IDs isolate the new collections from cached entries belonging to retired feeds. Public example assets are separate from users' private, seven-day media storage.

To update, review examples and their licenses at pinned commits, update the catalog and provenance together, retain author/license notices, and validate every local cover. Bump the catalog path for revisions so saved source signatures detect the update. Check migration from existing accounts, tag/search filters, copy/use in the image workbench, custom-source preservation, and offline loading before release.

Only administrators can configure prompt sources, the local proxy panel and WebDAV. The server supplies the settings capability from the authenticated role and rejects non-admin reads/writes of the prompt-source state. Ordinary accounts use the bundled catalogs without loading their legacy custom feeds; administrators’ custom sources remain account-scoped and are not published globally. Configuration import cannot restore these advanced settings for ordinary users. The old v1 public catalog is retained for already-open clients.
