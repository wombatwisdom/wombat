import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import asciidoc from "astro-asciidoc";

// https://astro.build/config
export default defineConfig({
	site:'https://wombatwisdom.github.io',
	integrations: [
		asciidoc({

		}),
		starlight({
			title: 'Wombat Docs',
			social: {
				github: 'https://github.com/wombatwisdom/wombat',
			},
			sidebar: [
				{
					slug: 'getting_started',
				},
				{
					label: 'Configuration',
					collapsed: true,
					autogenerate: {
						directory: 'configuration',
						collapsed: true,
					},
				},
				{
					label: 'Components',
					collapsed: true,
					autogenerate: {
						directory: 'components',
						collapsed: true,
					},
				},
				{
					label: 'Bloblang',
					collapsed: true,
					autogenerate: {
						directory: 'bloblang',
						collapsed: true,
					},
				},
				{
					label: 'Guides',
					collapsed: true,
					autogenerate: {
						directory: 'guides',
						collapsed: true,
					}
				},
				{
					label: 'Reference',
					collapsed: true,
					autogenerate: {
						directory: 'reference',
						collapsed: true,
					},
				},
			],
		}),
	],
});
