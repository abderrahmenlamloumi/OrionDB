import { themes as prismThemes } from "prism-react-renderer";
import type { Config } from "@docusaurus/types";
import type * as Preset from "@docusaurus/preset-classic";

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

const config: Config = {
  title: "OrionDB Documentation",
  tagline: "Telemetry ingestion and storage engine docs",
  //favicon: "",

  // Future flags, see https://docusaurus.io/docs/api/docusaurus-config#future
  future: {
    v4: true, // Improve compatibility with the upcoming Docusaurus v4
  },

  // Set the production url of your site here
  url: "https://abderrahmenlamloumi.github.io",
  // Set the /<baseUrl>/ pathname under which your site is served
  // For GitHub pages deployment, it is often '/<projectName>/'
  baseUrl: "/OrionDB/",
  trailingSlash: false,

  // GitHub pages deployment config.
  // If you aren't using GitHub pages, you don't need these.
  organizationName: "abderrahmenlamloumi",
  projectName: "OrionDB",

  onBrokenLinks: "throw",

  // Even if you don't use internationalization, you can use this field to set
  // useful metadata like html lang. For example, if your site is Chinese, you
  // may want to replace "en" with "zh-Hans".
  i18n: {
    defaultLocale: "en",
    locales: ["en"],
  },

  presets: [
    [
      "classic",
      {
        docs: {
          sidebarPath: "./sidebars.ts",
          editUrl:
            "https://github.com/abderrahmenlamloumi/OrionDB/tree/main/website/",
        },
        blog: false,
        theme: {
          customCss: "./src/css/custom.css",
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    // Replace with your project's social card
    colorMode: {
      defaultMode: "light",
      disableSwitch: true,
      respectPrefersColorScheme: false,
    },
    navbar: {
      title: "OrionDB Docs",
      logo: {
        alt: "OrionDB logo",
        src: 'img/OrionDbArchitecture.png',
      },
      items: [
        {
          type: "docSidebar",
          sidebarId: "docsSidebar",
          position: "left",
          label: "Documentation",
        },
        { to: "/docs/project-overview", label: "Overview", position: "left" },
        {
          to: "/docs/getting-started",
          label: "Getting started",
          position: "left",
        },
        {
          href: "https://github.com/abderrahmenlamloumi/OrionDB",
          label: "GitHub",
          position: "right",
        },
      ],
    },
    footer: {
      style: "dark",
      links: [
        {
          title: "Docs",
          items: [
            {
              label: "Project overview",
              to: "/docs/project-overview",
            },
            {
              label: "Architecture",
              to: "/docs/architecture-concepts",
            },
          ],
        },
        {
          title: "Project",
          items: [
            {
              label: "Repository",
              href: "https://github.com/abderrahmenlamloumi/OrionDB",
            },
            {
              label: "Main README",
              href: "https://github.com/abderrahmenlamloumi/OrionDB#readme",
            },
            {
              label: "orion-db module",
              href: "https://github.com/abderrahmenlamloumi/OrionDB/tree/main/orion-db",
            },
          ],
        },
      ],
      //copyright: `Copyright © ${new Date().getFullYear()} OrionDB contributors.`,
    },
    prism: {
      theme: prismThemes.github,
      additionalLanguages: [
        "bash",
        "go",
        "protobuf",
        "python",
        "json",
        "yaml",
        "toml",
        "hcl",
      ],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
