import type { ReactNode } from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';

import styles from './index.module.css';

function HomepageHeader() {
  const { siteConfig } = useDocusaurusContext();
  return (
    <header className={clsx('hero hero--primary', styles.heroBanner)}>
      <div className="container">
        <Heading as="h1" className="hero__title">
          OrionDB Documentation
        </Heading>
        <p className="hero__subtitle">{siteConfig.tagline}</p>
        <div className={styles.buttons}>
          <Link
            className="button button--secondary button--lg"
            to="/docs/getting-started">
            Get started
          </Link>
          <Link className="button button--outline button--lg" to="/docs/architecture-concepts">
            Architecture
          </Link>
        </div>
      </div>
    </header>
  );
}

export default function Home(): ReactNode {
  return (
    <Layout
      title="Home"
      description="Official documentation site for OrionDB and the orion-db module.">
      <HomepageHeader />
      <main>
        <section className={styles.summarySection}>
          <div className="container">
            <div className={styles.grid}>
              <article className={styles.card}>
                <Heading as="h2">What OrionDB is</Heading>
                <p>
                  OrionDB is an experimental telemetry ingestion and storage project implemented in Go,
                  designed to explore high-throughput pipelines, cardinality-aware indexing, and
                  append-only WAL-based persistence.
                </p>
              </article>
              <article className={styles.card}>
                <Heading as="h2">Start paths</Heading>
                <ul>
                  <li>
                    <Link to="/docs/first-working-example">Run a first end-to-end example</Link>
                  </li>
                  <li>
                    <Link to="/docs/api-command-reference">Check commands and gRPC schema</Link>
                  </li>
                  <li>
                    <Link to="/docs/task-oriented-guides">Use task-oriented workflows</Link>
                  </li>
                </ul>
              </article>
            </div>
          </div>
        </section>
      </main>
    </Layout>
  );
}
