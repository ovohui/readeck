/**
 * @returns {boolean}
 */
exports.isActive = function () {
  return $.host == "news.ycombinator.com";
};

/**
 * @param {Node} document
 */
exports.documentLoaded = function (document) {
  const body = document.body;
  if (!body) throw "document body missing";

  const bigbox = document.querySelector("#bigbox");
  if (!bigbox) return;

  const fatitem = bigbox.querySelector(".fatitem");
  body.append(fatitem.querySelector(".submission .titleline > a[href]"));
  const byline = document.createElement("p");
  const hnuser = fatitem.querySelector(".hnuser");
  const age = fatitem.querySelector(".age[title]");
  byline.append("posted by ", hnuser, " on ", ageToTimeElement(document, age));
  body.append(byline);

  const toptext = fatitem.querySelector(".toptext");
  if (toptext) body.append(toptext);

  const comments = bigbox.querySelectorAll(".commtext");
  const commentsHeading = document.createElement("h2");
  commentsHeading.append(`${comments.length} comments`);
  body.append(commentsHeading);

  comments.forEach((commtext) => {
    const comtr = commtext.closest(".comtr");
    const hnuser = comtr.querySelector(".comhead .hnuser");
    const age = comtr.querySelector(".comhead .age[title]");
    const itemlink = age.querySelector("a[href]");
    itemlink.firstChild?.remove();
    itemlink.append(ageToTimeElement(document, age));
    const byline = document.createElement("p");
    byline.append(hnuser, " • ", itemlink);
    body.append(byline, commtext, document.createElement("hr"));
  });

  bigbox.closest("table").remove();
  $.readability = false;
};

/**
 * @param {Node} document
 * @param {Node} age
 * @returns Element
 */
function ageToTimeElement(document, age) {
  const ts = age.getAttribute("title").replace(/ \d+$/, "");
  const time = document.createElement("time");
  time.setAttribute("datetime", ts);
  time.append(ts.replace("T", " ").replace(/:\d{2}$/, ""));
  return time;
}
