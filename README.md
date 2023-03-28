# gluestick
gluing the web together on one page


## Build
```
cd ./server/
go fmt ./... && go vet ./... && go build gluestick.go
```

Then run via `./gluestick`


## Sample Scrape Requests

```
curl -v "http://localhost:6100/scrape" --data '
{
    "url": "https://thedrive.com/the-war-zone",
    "items": {
        "articles": {
            "selector": "div.category-articles article",
            "fields": {
                "title": "h3",
                "summary": "h3 ~ p",
                "author": "div.MuiBox-root.css-10ctbcu > div.MuiBox-root.css-19idom > a",
                "date": "div.MuiBox-root.css-10ctbcu > p",
                "link": "a.MuiBox-root:first-child|href",
                "image": {
                    "src": "a img|src",
                    "alt": "a img|alt",
                    "title": "a img|title"
                }
            }
        }
    }
}'
```

Which gets you:

```
{
    "articles": [
        {
            "author": "Emma Helfrich",
            "date": "Mar 28, 2023",
            "image": {
                "alt": "Six F-16s Getting Autonomous Computer Brains For Combat Drone Trials",
                "src": "https://www.thedrive.com/uploads/2023/03/28/121206-F-ZZ999-533.jpg?auto=webp\u0026crop=16%3A9\u0026auto=webp\u0026optimize=high\u0026quality=70\u0026width=1440",
                "title": "Six F-16s Getting Autonomous Computer Brains For Combat Drone Trials"
            },
            "link": "/the-war-zone/six-f-16s-getting-autonomous-computer-brains-for-combat-drone-trials",
            "summary": "Dubbed Project VENOM, the findings gathered under the effort will feed into the service’s overarching Collaborative Combat Aircraft program.",
            "title": "Six F-16s Getting Autonomous Computer Brains For Combat Drone Trials"
        },
        {
            "author": "Howard Altman",
            "date": "Mar 27, 2023",
            "image": {
                "alt": "Ukraine Situation Report: Challenger Tanks, Stryker Armored Vehicles Arrive In Country",
                "src": "https://www.thedrive.com/uploads/2023/03/28/Armor-arrives-Ukraine.jpg?auto=webp\u0026crop=16%3A9\u0026auto=webp\u0026optimize=high\u0026quality=70\u0026width=1440",
                "title": "Ukraine Situation Report: Challenger Tanks, Stryker Armored Vehicles Arrive In Country"
            },
            "link": "/the-war-zone/ukraine-situation-report-challenger-tanks-stryker-armored-vehicles-arrive-in-country",
            "summary": "With the arrival of the new Western tanks and other armor, Ukraine’s Defense Minister said “our military zoo is expanding.”",
            "title": "Ukraine Situation Report: Challenger Tanks, Stryker Armored Vehicles Arrive In Country"
        },

        ...
    ]
}

```


## Scrape Request Format
```
{
    "url": "http://example.com",
    "items": {
        "something": { "selector": "div.someclass", fields: { <fields here> }},
        "anotherThing": { "selector": "div#foo", fields: { <fields here> }},
        // etc...
    }
}
```

You must provide a `url` and `items`.  Items are name-to-`{selector, field}` object.

The `selector` is a CSS selector which is the anchor from which the field's values are extracted.

The `fields` are name-to-values where values are either:

* A string value selector (see format below)
* Another object which itself can be comprised of nested value selectors. See the `articles.image` in the examples.

### Value Selectors
Using the parent as a starting point, extract values according to:

```
[css-selector][|attribute]
```

Ex:

```
"a img|src"
```

`|attribute` is optional, if omitted, all text is scraped witin selected element.

If the `css-selector` is blank, the `attribute` is taken on the containing parent selector.  This is useful if you want to select multiple attributes from the same item. Ex:

```
{
    "url": "https://www.example.com/some-article.html",
    "items": {
        "articles": {
            "selector": "div#article-wrapper article",
            "fields": {
                "image": {
                    "src": "a img|src"
                    "alt": "a img|alt"
                },
                "all_text": ""
                "links": "a.someclass|href"
            }
        }
    }
}
```

In this example `all_text` is taking the all text of the parent article tag (`div#article-wrapper article`)--as neither css-selector or attribute was specified so it selects the containing/parent and uses the text respectively.

### Single vs Multi Valued Fields
You'll get back either a single string value or an array of string values depending on how many times your value selector was matched in the DOM.  You may have to be more restrictive in your selectors or use `:first-child` and other pseedo classes to limit overzealosu value capturing.


## Tips for Field Selectors
Select the desired part of the DOM in your browser's `Dev Tools` and right-click `Copy > Copy Selector`. Then modify as desired based on parent selector--for example, you may need to remove the first `n` parts of the selector as it will be global/from the root of the DOM, not from your parent's selector.

## References
* [colly godoc](https://pkg.go.dev/github.com/gocolly/colly) - scraping library
* [goquery godoc](https://pkg.go.dev/github.com/PuerkitoBio/goquery) - used for element selection & manipulation
* [cascadia godoc](https://github.com/andybalholm/cascadia) - is what parses the css selector strings
