import { MobileMenu } from './MobileMenu';
import { SocialIcons } from './SocialIcons';

/** Family navbar (copied from vaicorinthians.lauraesteves.com): logo left, social icons right, room for the GitHub corner. */
export function Navbar() {
  return (
    <nav
      aria-label="Principal"
      className="fixed top-0 right-0 left-0 z-50 bg-bg/80 backdrop-blur-sm"
    >
      <div className="relative flex items-center px-6 py-4 pr-[100px]">
        <a
          href="/"
          className="gradient-text font-mono text-xl font-bold no-underline transition-opacity hover:opacity-80 md:text-2xl"
        >
          notas
        </a>

        <div className="ml-auto hidden items-center md:flex">
          <SocialIcons />
        </div>

        <div className="ml-auto md:hidden">
          <MobileMenu />
        </div>
      </div>
    </nav>
  );
}
