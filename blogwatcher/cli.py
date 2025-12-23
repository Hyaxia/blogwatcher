"""CLI commands for BlogWatcher."""

import sqlite3
from typing import Optional

import click

from .db import Database
from .models import Blog
from .scanner import scan_all_blogs, scan_blog_by_name


@click.group()
@click.version_option()
def cli():
    """BlogWatcher - Track blog articles and detect new posts."""
    pass


@cli.command()
@click.argument("name")
@click.argument("url")
@click.option("--feed-url", help="RSS/Atom feed URL (auto-discovered if not provided)")
@click.option("--scrape-selector", help="CSS selector for HTML scraping fallback")
def add(name: str, url: str, feed_url: Optional[str], scrape_selector: Optional[str]):
    """Add a new blog to track."""
    db = Database()
    try:
        # Check if blog with same name exists
        if db.get_blog_by_name(name):
            click.echo(click.style(f"Error: Blog '{name}' already exists", fg="red"))
            raise SystemExit(1)

        # Check if blog with same URL exists
        if db.get_blog_by_url(url):
            click.echo(click.style(f"Error: Blog with URL '{url}' already exists", fg="red"))
            raise SystemExit(1)

        blog = Blog(
            id=None,
            name=name,
            url=url,
            feed_url=feed_url,
            scrape_selector=scrape_selector,
        )
        db.add_blog(blog)
        click.echo(click.style(f"Added blog '{name}'", fg="green"))
    except sqlite3.IntegrityError:
        click.echo(click.style(f"Error: Blog with URL '{url}' already exists", fg="red"))
        raise SystemExit(1)
    finally:
        db.close()


@cli.command()
@click.argument("name")
@click.option("--yes", "-y", is_flag=True, help="Skip confirmation prompt")
def remove(name: str, yes: bool):
    """Remove a blog from tracking."""
    db = Database()
    try:
        blog = db.get_blog_by_name(name)
        if not blog:
            click.echo(click.style(f"Error: Blog '{name}' not found", fg="red"))
            raise SystemExit(1)

        if not yes:
            click.confirm(
                f"Remove blog '{name}' and all its articles?",
                abort=True,
            )

        db.remove_blog(blog.id)
        click.echo(click.style(f"Removed blog '{name}'", fg="green"))
    finally:
        db.close()


@cli.command("list-blogs")
def list_blogs():
    """List all tracked blogs."""
    db = Database()
    try:
        blogs = db.list_blogs()
        if not blogs:
            click.echo("No blogs tracked yet. Use 'blogwatcher add' to add one.")
            return

        click.echo(click.style(f"Tracked blogs ({len(blogs)}):", fg="cyan", bold=True))
        click.echo()

        for blog in blogs:
            click.echo(click.style(f"  {blog.name}", fg="white", bold=True))
            click.echo(f"    URL: {blog.url}")
            if blog.feed_url:
                click.echo(f"    Feed: {blog.feed_url}")
            if blog.scrape_selector:
                click.echo(f"    Selector: {blog.scrape_selector}")
            if blog.last_scanned:
                click.echo(f"    Last scanned: {blog.last_scanned.strftime('%Y-%m-%d %H:%M')}")
            click.echo()
    finally:
        db.close()


@cli.command()
@click.argument("blog_name", required=False)
def scan(blog_name: Optional[str]):
    """Scan blogs for new articles.

    If BLOG_NAME is provided, only that blog is scanned.
    Otherwise, all blogs are scanned.
    """
    db = Database()
    try:
        if blog_name:
            # Scan specific blog
            result = scan_blog_by_name(db, blog_name)
            if result is None:
                click.echo(click.style(f"Error: Blog '{blog_name}' not found", fg="red"))
                raise SystemExit(1)

            _print_scan_result(result)
        else:
            # Scan all blogs
            blogs = db.list_blogs()
            if not blogs:
                click.echo("No blogs tracked yet. Use 'blogwatcher add' to add one.")
                return

            click.echo(click.style(f"Scanning {len(blogs)} blog(s)...", fg="cyan"))
            click.echo()

            results = scan_all_blogs(db)
            total_new = 0

            for result in results:
                _print_scan_result(result)
                total_new += result.new_articles

            click.echo()
            if total_new > 0:
                click.echo(
                    click.style(f"Found {total_new} new article(s) total!", fg="green", bold=True)
                )
            else:
                click.echo(click.style("No new articles found.", fg="yellow"))
    finally:
        db.close()


def _print_scan_result(result):
    """Print a single scan result."""
    status_color = "green" if result.new_articles > 0 else "white"

    click.echo(click.style(f"  {result.blog_name}", fg="white", bold=True))

    if result.error:
        click.echo(click.style(f"    Error: {result.error}", fg="red"))
    elif result.source == "none":
        click.echo(click.style("    No feed or scraper configured", fg="yellow"))
    else:
        source_label = "RSS" if result.source == "rss" else "HTML"
        click.echo(
            f"    Source: {source_label} | "
            f"Found: {result.total_found} | "
            + click.style(f"New: {result.new_articles}", fg=status_color)
        )


@cli.command()
@click.option("--all", "-a", "show_all", is_flag=True, help="Show all articles (including read)")
@click.option("--blog", "-b", "blog_name", help="Filter by blog name")
def articles(show_all: bool, blog_name: Optional[str]):
    """List articles.

    By default, shows only unread articles.
    """
    db = Database()
    try:
        blog_id = None
        if blog_name:
            blog = db.get_blog_by_name(blog_name)
            if not blog:
                click.echo(click.style(f"Error: Blog '{blog_name}' not found", fg="red"))
                raise SystemExit(1)
            blog_id = blog.id

        articles_list = db.list_articles(unread_only=not show_all, blog_id=blog_id)

        if not articles_list:
            if show_all:
                click.echo("No articles found.")
            else:
                click.echo(click.style("No unread articles!", fg="green"))
            return

        # Get blog names for display
        blogs = {b.id: b.name for b in db.list_blogs()}

        label = "All articles" if show_all else "Unread articles"
        click.echo(click.style(f"{label} ({len(articles_list)}):", fg="cyan", bold=True))
        click.echo()

        for article in articles_list:
            _print_article(article, blogs.get(article.blog_id, "Unknown"))
    finally:
        db.close()


def _print_article(article, blog_name: str):
    """Print a single article."""
    status = click.style("[read]", fg="bright_black") if article.is_read else click.style("[new]", fg="yellow")
    id_str = click.style(f"[{article.id}]", fg="cyan")

    click.echo(f"  {id_str} {status} {article.title}")
    click.echo(f"       Blog: {blog_name}")
    click.echo(f"       URL: {article.url}")
    if article.published_date:
        click.echo(f"       Published: {article.published_date.strftime('%Y-%m-%d')}")
    click.echo()


@cli.command()
@click.argument("article_id", type=int)
def read(article_id: int):
    """Mark an article as read."""
    db = Database()
    try:
        article = db.get_article(article_id)
        if not article:
            click.echo(click.style(f"Error: Article {article_id} not found", fg="red"))
            raise SystemExit(1)

        if article.is_read:
            click.echo(f"Article {article_id} is already marked as read.")
            return

        db.mark_article_read(article_id)
        click.echo(click.style(f"Marked article {article_id} as read", fg="green"))
    finally:
        db.close()


@cli.command("read-all")
@click.option("--blog", "-b", "blog_name", help="Only mark articles from this blog")
@click.option("--yes", "-y", is_flag=True, help="Skip confirmation prompt")
def read_all(blog_name: Optional[str], yes: bool):
    """Mark all unread articles as read."""
    db = Database()
    try:
        blog_id = None
        if blog_name:
            blog = db.get_blog_by_name(blog_name)
            if not blog:
                click.echo(click.style(f"Error: Blog '{blog_name}' not found", fg="red"))
                raise SystemExit(1)
            blog_id = blog.id

        articles_list = db.list_articles(unread_only=True, blog_id=blog_id)

        if not articles_list:
            click.echo(click.style("No unread articles to mark as read.", fg="green"))
            return

        if not yes:
            scope = f"from '{blog_name}'" if blog_name else "all blogs"
            click.confirm(
                f"Mark {len(articles_list)} article(s) {scope} as read?",
                abort=True,
            )

        for article in articles_list:
            db.mark_article_read(article.id)

        click.echo(click.style(f"Marked {len(articles_list)} article(s) as read", fg="green"))
    finally:
        db.close()


@cli.command()
@click.argument("article_id", type=int)
def unread(article_id: int):
    """Mark an article as unread."""
    db = Database()
    try:
        article = db.get_article(article_id)
        if not article:
            click.echo(click.style(f"Error: Article {article_id} not found", fg="red"))
            raise SystemExit(1)

        if not article.is_read:
            click.echo(f"Article {article_id} is already marked as unread.")
            return

        db.mark_article_unread(article_id)
        click.echo(click.style(f"Marked article {article_id} as unread", fg="green"))
    finally:
        db.close()


if __name__ == "__main__":
    cli()
