import type {ReactNode} from 'react';
import Link from '@docusaurus/Link';
import Heading from '@theme/Heading';
import styles from './homepage.module.css';

type Capability = {
  name: string;
  summary: string;
  href: string;
};

const capabilities: Capability[] = [
  {
    name: 'catalog',
    summary: 'List and inspect available tools from all discovered sources.',
    href: '/docs/cli/catalog-and-explain',
  },
  {
    name: 'explain',
    summary: 'Show schema, description, and parameter detail for any tool.',
    href: '/docs/cli/catalog-and-explain',
  },
  {
    name: 'workflow',
    summary: 'Run multi-step operations with declarative workflow definitions.',
    href: '/docs/cli/workflow-run',
  },
  {
    name: 'dynamic-tool',
    summary: 'Execute individual OpenAPI operations directly from the CLI.',
    href: '/docs/cli/tool-execution',
  },
  {
    name: 'MCP server',
    summary:
      'Expose the governed catalog over the Model Context Protocol for agent use.',
    href: '/docs/cli/overview',
  },
];

export default function CapabilityStrip(): ReactNode {
  return (
    <section className={styles.capabilitySection} aria-labelledby="capability-strip-heading">
      <div className="container">
        <Heading as="h2" id="capability-strip-heading" className={styles.capabilityHeading}>
          Command surface
        </Heading>
        <ul className={styles.capabilityList} role="list">
          {capabilities.map((cap) => (
            <li key={cap.name} className={styles.capabilityItem}>
              <Link to={cap.href} className={styles.capabilityName}>
                <code>{cap.name}</code>
              </Link>
              <span className={styles.capabilitySummary}>{cap.summary}</span>
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
}
