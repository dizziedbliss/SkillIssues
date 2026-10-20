import { Link } from "react-router";
import tokens from "./tokens.module.css";
import s from "./Landing.module.css";
import { Header } from "./Header";
import { Footer } from "./Footer";
import { LiveBar } from "./LiveBar";
import { GlyphNoise } from "./GlyphNoise";
import { Logo } from "./Logo";
import { external } from "../lib/links";

/**
 * The landing page, as the owner designed it: one centred column with equal
 * margins, white space left white, over a faint glyph-noise background. Its
 * look is locked (tokens.module.css); themes never reach it.
 */
export function Landing() {
  return (
    <div className={`${tokens.page} ${s.page}`}>
      <GlyphNoise />
      <Header />
      <main className={s.column}>
        <section className={s.hero} aria-labelledby="hero-title">
          <h1 id="hero-title" className={s.heading}>Better software doesn’t happen on its&nbsp;own.</h1>
          <p className={s.body}>
            Across open source, millions of issues are waiting: bugs to fix, improvements to make, ideas to
            bring to life. That’s unfinished work in the software we all rely on, and the pile keeps growing.
          </p>
          <LiveBar />
        </section>

        <section className={s.section} aria-labelledby="us-title">
          <h2 id="us-title" className={s.heading}>It’s a lot of issues. And there are a lot of us.</h2>
          <p className={s.body}>
            Every project has work waiting to be done: bugs to fix, improvements to make, and questions to
            answer. Across thousands of projects, those issues add up faster than any one person can tackle
            them.
          </p>
          <p className={s.body}>
            But it doesn’t have to be one person. Big challenges move when people take useful steps together.
            Here, that can start with one issue you know how to solve.
          </p>
        </section>

        <section className={`${s.section} ${s.centred}`} aria-labelledby="made-title">
          <p className={s.made}>
            So we made <Logo size="inline" />
          </p>
          <h2 id="made-title" className={s.heading}>A global initiative to help tackle open source’s growing backlog.</h2>
          <p className={s.body}>
            SkillIssues brings open issues from active GitHub projects into one searchable place. Find one that
            fits your language, skills, and experience—then head to GitHub and help make it better.
          </p>
          <Link to="/app" className={s.button}>Explore open issues</Link>
        </section>

        <section className={s.section} aria-labelledby="dev-title">
          <p className={s.kicker}>For developers</p>
          <h2 id="dev-title" className={s.heading}>Find an issue that feels like yours.</h2>
          <p className={s.body}>
            Maybe you’re learning a new language, looking for your first contribution, or ready to take on
            something more challenging. Choose what you’re interested in, explore issues by skill and
            difficulty, and find a place to start.
          </p>
          <Link to="/app" className={s.button}>Solve issues</Link>
        </section>

        <section className={s.section} aria-labelledby="docs-title">
          <h2 id="docs-title" className={s.heading}>A good place to start—and help along the way.</h2>
          <p className={s.body}>
            Every project does things a little differently. Our docs will help you get set up, understand the
            work, and find your way from “I could help with this” to opening a pull request.
          </p>
        </section>

        <section className={s.section} aria-labelledby="maint-title">
          <p className={s.kicker}>For maintainers</p>
          <h2 id="maint-title" className={s.heading}>Your project’s next contributor could be out there looking for you.</h2>
          <p className={s.body}>
            SkillIssues helps connect open issues with developers interested in your project’s languages and
            skills. So more people can find the work that needs doing—and your project can keep moving forward.
          </p>
          <Link to="/maintainers" className={s.button}>Learn how to include your project</Link>
        </section>

        <section className={s.section} aria-labelledby="desk-title">
          <h2 id="desk-title" className={s.heading}>Keep the contribution moving.</h2>
          <p className={s.body}>
            Found your issue? The desktop app helps you get into the work: pull the project, make your changes,
            push them, and keep track of your pull requests.
          </p>
          <p className={s.body}>Less time juggling steps. More time making something better.</p>
          <p className={s.ctaLine}>
            <Link to="/download" className={s.button}>Download the desktop app</Link>
            <span className={s.aside}>Available for Windows, macOS and Linux.</span>
          </p>
        </section>

        <section className={s.section} aria-labelledby="effort-title">
          <h2 id="effort-title" className={s.heading}>Every contribution adds to the bigger effort.</h2>
          <p className={s.body}>
            We’re working on levels and a community leaderboard to celebrate the people helping move open
            source forward—and show what we can accomplish together.
          </p>
        </section>

        <section className={s.section} aria-labelledby="trust-title">
          <p className={s.kicker}>Built with trust in mind</p>
          <h2 id="trust-title" className={s.heading}>Know what’s happening with your account and code.</h2>
          <p className={s.body}>
            Contributing should feel good—and that includes knowing how your tools work. Our security docs
            explain how SkillIssues handles your account, your code, and the contribution process.
          </p>
          <Link to="/security" className={s.button}>Read the security docs</Link>
        </section>

        <section className={s.section} aria-labelledby="live-title">
          <h2 id="live-title" className={s.heading}>And you can see how SkillIssues works, too.</h2>
          <p className={s.body}>
            We wanted the product to be open about what’s happening behind the scenes. SkillIssues Live lets
            you watch activity move through the system and see the different parts working together.
          </p>
          <a href={external.live} className={s.button}>Explore SkillIssues Live</a>
        </section>

        <section className={`${s.closing} ${s.centred}`} aria-labelledby="end-title">
          <h2 id="end-title" className={s.heading}>There will always be more work to do. Start with one issue.</h2>
          <p className={s.body}>Find something you care about. Make a contribution. Help a project take its next step.</p>
          <Link to="/app" className={s.button}>[Find an issue]</Link>
        </section>
      </main>
      <Footer />
    </div>
  );
}
