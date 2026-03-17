import type {ReactNode} from 'react';
import Link from '@docusaurus/Link';
import Heading from '@theme/Heading';
import styles from './docs-components.module.css';

export type AudienceLink = {
  label: string;
  href: string;
};

export type AudienceCard = {
  id: string;
  rung: number;
  title: string;
  audience: string;
  description: string;
  links: AudienceLink[];
};

const defaultCards: AudienceCard[] = [
  {
    id: 'first-run',
    rung: 1,
    title: 'First run',
    audience: 'End users · agent authors',
    description:
      'You want commands that work. Start here to understand the two binaries, choose a mode, build them, and run your first tool.',
    links: [
      {label: 'Introduction', href: '/docs/getting-started/intro'},
      {label: 'Choose your path', href: '/docs/getting-started/choose-your-path'},
      {label: 'Installation', href: '/docs/getting-started/installation'},
      {label: 'Quickstart', href: '/docs/getting-started/quickstart'},
      {label: 'CLI overview', href: '/docs/cli/overview'},
    ],
  },
  {
    id: 'runtime-depth',
    rung: 2,
    title: 'Runtime depth',
    audience: 'Operators · developers',
    description:
      'You are configuring a reusable runtime, hardening a deployment, or wiring up auth, policy, and overlays.',
    links: [
      {label: 'Runtime overview', href: '/docs/runtime/overview'},
      {label: 'Deployment models', href: '/docs/runtime/deployment-models'},
      {label: 'Configuration overview', href: '/docs/configuration/overview'},
      {label: 'Discovery & Catalog', href: '/docs/discovery-catalog/overview'},
      {label: 'Security overview', href: '/docs/security/overview'},
      {label: 'Operations overview', href: '/docs/operations/overview'},
    ],
  },
  {
    id: 'enterprise',
    rung: 3,
    title: 'Enterprise evaluation',
    audience: 'Security reviewers · procurement',
    description:
      'You need a reviewable evidence package: real auth proof, reproducible test artifacts, auditability, and an honest account of known gaps.',
    links: [
      {label: 'Enterprise overview', href: '/docs/enterprise/overview'},
      {label: 'Adoption checklist', href: '/docs/enterprise/adoption-checklist'},
      {label: 'Enterprise readiness', href: '/docs/runtime/enterprise-readiness'},
      {label: 'Authentik reference proof', href: '/docs/runtime/authentik-reference'},
      {label: 'Fleet validation', href: '/docs/development/fleet-validation'},
    ],
  },
];

type Props = {
  cards?: AudienceCard[];
};

export default function AudienceCards({cards = defaultCards}: Props): ReactNode {
  return (
    <section aria-labelledby="audience-cards-heading">
      <Heading as="h2" id="audience-cards-heading" className={styles.visuallyHiddenHeading}>
        Audience paths
      </Heading>
      <div className={styles.grid}>
        {cards.map((card) => (
          <article key={card.id} className={styles.audienceCard}>
            <header className={styles.cardHeader}>
              <span className={styles.rungBadge} aria-label={`Rung ${card.rung}`}>
                {card.rung}
              </span>
              <Heading as="h3" className={styles.cardTitle}>
                {card.title}
              </Heading>
            </header>
            <p className={styles.audience}>{card.audience}</p>
            <p className={styles.description}>{card.description}</p>
            <nav aria-label={`${card.title} navigation`}>
              <ul className={styles.linkList}>
                {card.links.map((link) => (
                  <li key={link.href}>
                    <Link to={link.href}>{link.label}</Link>
                  </li>
                ))}
              </ul>
            </nav>
          </article>
        ))}
      </div>
    </section>
  );
}
