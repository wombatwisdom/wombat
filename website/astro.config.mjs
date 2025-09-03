import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import asciidoc from "astro-asciidoc";
import starlightLinksValidator from 'starlight-links-validator'
import rehypeMermaid from 'rehype-mermaid'

// https://astro.build/config
export default defineConfig({
	site:'https://wombatwisdom.github.io',
	image: {
		// Use the passthrough service which doesn't process images
		service: {
			entrypoint: 'astro/assets/services/noop',
		},
	},
	markdown: {
		rehypePlugins: [
			[rehypeMermaid, { simple: true }],
		],
	},
	plugins: [
		starlightLinksValidator(),
	],
	integrations: [
		asciidoc({

		}),
		starlight({
			title: 'Wombat',
			social: {
				github: 'https://github.com/wombatwisdom/wombat',
			},
			customCss: [
				// Relative path to your custom CSS file
				'./src/styles/custom.css',
			],
			sidebar: [
				{
					slug: 'getting_started',
				},
				{
					label: 'Pipelines',
					collapsed: true,
					items: [
						{
							label: 'Learn',
							collapsed: true,
							autogenerate: {
								directory: 'pipelines/learn',
								collapsed: true,
							}
						},
						{
							label: 'Build',
							collapsed: true,
							autogenerate: {
								directory: 'pipelines/build',
								collapsed: true,
							}
						},
						{
							label: 'Debug',
							collapsed: true,
							autogenerate: {
								directory: 'pipelines/debug',
								collapsed: true,
							}
						},
						{
							label: 'Test',
							collapsed: true,
							autogenerate: {
								directory: 'pipelines/test',
								collapsed: true,
							}
						},
						{
							label: 'Deploy',
							collapsed: true,
							autogenerate: {
								directory: 'pipelines/deploy',
								collapsed: true,
							}
						},
						{
							label: 'Observe',
							collapsed: true,
							autogenerate: {
								directory: 'pipelines/observe',
								collapsed: true,
							}
						},
					]
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
