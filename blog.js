// Blog posts data
const blogPosts = [

    {
        date: "August 9, 2025",
        tag: "Programming",
        title: "My Solution To LeetCode 231: Power Of Two",
        summary: "This is my approach on solving today’s LeetCode, by using Logarithm and bitwise operations. I will use C# to demonstrate the solutions.",
        link: "https://wholesale-bowler-01e.notion.site/My-Solution-To-LeetCode-231-Power-Of-Two-24a3fbb9906a8093ba64c72081c6152d",
        thumbnail: "images/leetcode.png"
    },

    {
        date: "August 1, 2025",
        tag: "Technology",
        title: "What Is An Operating System?",
        summary: "An operating system is the backbone of any computing device, quietly managing hardware, running applications, and enabling users to interact with their machines. Whether you're using a smartphone, laptop, or server, the operating system handles everything from memory and processes to files and security. In this blog post, we'll explore what an operating system is, how it works, and why it's a vital part of modern technology.",
        link: "https://wholesale-bowler-01e.notion.site/What-Is-An-Operating-System-2423fbb9906a808381d0fed3d355665c",
        thumbnail: "images/operating-system.jpg" // Add your thumbnail path here
    },

    {
        date: "July 29, 2025",
        tag: "Music",
        title: "The Bittersweet Beauty Of Youth In “Sakurazuki”",
        summary: "“Sakurazuki” is the fifth single by Sakurazaka46, featuring lyrics by Yasushi Akimoto and another memorable composition from Nazca. In my opinion, it's the group's most emotionally resonant tracks, weaving together the universal themes of first love, growing up, and the painful beauty of letting go.",
        link: "https://wholesale-bowler-01e.notion.site/The-Bittersweet-Beauty-Of-Youth-In-Sakurazuki-23f3fbb9906a8079b760f1082eead152",
        thumbnail: "images/sakurazuki-thumbnail.jpg" // Add your thumbnail path here
    },

    {
        date: "July 25, 2025",
        tag: "Artificial Intelligence",
        title: "Understanding AI: Clarifying Concepts Behind Everyday Interactions",
        summary: "We interact with AI systems daily, from asking ChatGPT questions to getting recommendations on streaming platforms. But despite this familiarity, there might be some struggle in understanding what artificial intelligence actually is and how it works. ",
        link: "https://wholesale-bowler-01e.notion.site/Understanding-AI-Clarifying-Concepts-Behind-Everyday-Interactions-23b3fbb9906a80c7a490d03eff474ee2",
        thumbnail: "images/ai-thumbnail.jpg"
    },

    {
        date: "July 22, 2025",
        tag: "Gaming, Surrealism",
        title: "Miserere:  Echoes Beyond the Canvas of Reality",
        summary: "Miserere is a haunting RPG Maker fangame that explore isolation, identity, and trauma through the fragmented dreams of a half-alien astronaut. Like Allegri's sacred composition, Miserere serves as a desperate plea for mercy in a universe of profound loneliness.",
        link: "https://wholesale-bowler-01e.notion.site/Miserere-Echoes-Beyond-the-Canvas-of-Reality-2383fbb9906a803fa284c2261e2427a7",
        thumbnail: "images/miserere.png"
    },

    {
        date: "July 17, 2025",
        tag: "Music",
        title: "The Art Of Leitmotif And Its Role in Undertale's Storytelling",
        summary: "Music is at the heart of Undertale's narrative, acting as more than just background ambiance. Each track is carefully crafted to reflect characters' personalities, key emotional moments, and the overall mood of the game's world. Especially, the use of leitmotif has helped to build emotional continuity and deepens the player's connection to the story.",
        link: "https://wholesale-bowler-01e.notion.site/The-Art-Of-Leitmotif-And-Its-Role-in-Undertale-s-Storytelling-2323fbb9906a80e0bd0ecaed6ab67c34",
        thumbnail: "images/undertale.webp"
    },

    {
        date: "July 12, 2025",
        tag: "Tech",
        title: "Model Context Protocol: The New Standard Reshaping API Development for the AI Era",
        summary: "Introduced by Anthropic in November 2024, Model Context Protocol, or MCP for short, has provided a new standard for AI assistants to connect to data systems.",
        link: "https://wholesale-bowler-01e.notion.site/Model-Context-Protocol-The-New-Standard-Reshaping-API-Development-for-the-AI-Era-22e3fbb9906a80d1bcbbcb251696663d",
        thumbnail: "images/mcp.jpg"
    },

    {
        date: "July 10, 2025",
        tag: "NES, System Architecture",
        title: "The System Architecture of NES",
        summary: "In this blog, I will take a closer look at the NES, one of the most iconic gaming consoles to ever release and explore how its hardware worked.",
        link: "https://wholesale-bowler-01e.notion.site/The-System-Architecture-of-NES-22b3fbb9906a80729557ceec2ff16663",
        thumbnail: "images/nes.jpg"
    },

    {
        date: "July 7, 2025",
        tag: "Programming",
        title: "My Solution to LeetCode 1353: Maximum Number of Events That Can Be Attended",
        summary: "This is my approach to solving LeetCode problem 1353. I'll use C# to demonstrate the solution, but the concepts can be applied in any programming language.",
        link: "https://wholesale-bowler-01e.notion.site/My-Solution-to-LeetCode-1353-Maximum-Number-of-Events-That-Can-Be-Attended-2293fbb9906a802ab787e087c36fb04d",
        thumbnail: "images/leetcode.png"
    },

    {
        date: "July 6, 2025",
        tag: "Programming",
        title: "Asynchronous Programming: From Callbacks to  Async/Await",
        summary: "In this blog post, I’ll try my best to cover the history of asynchronous programming, as well as its fundamental concepts, and practical applications.",
        link: "https://wholesale-bowler-01e.notion.site/Asynchronous-Programming-From-Callbacks-to-Async-Await-2283fbb9906a80e4abb9d30f3b98dd29",
        thumbnail: "images/async.png"
    },

    {
        date: "July 1, 2025",
        tag: "Physics",
        title: "Time Crystal: A Phase Of Matter That Seems To Defy Physics",
        summary: "What is Time Crystal? Short answer, a phase of matter in which its structural atoms still moving in repetitive motion even in its lowest energy state. Long answer, this entire blog post.",
        link: "https://wholesale-bowler-01e.notion.site/Time-Crystal-A-Phase-Of-Matter-That-Seems-To-Defy-Physics-2273fbb9906a8081b9a9f0c06299ad7d"
    },

    {
        date: "June 10, 2025",
        tag: "Music",
        title: "Cloudier and The Story Beneath Their Songs (Part 1)",
        summary: "A deeper dive into the EDM group band Cloudier, from their lyrical content to the narrative lurking behind each song. Disclaimer: This analysis is just from my perspective, and not everything in this post is 100% correct or claimed to be the intention of the artists in the first place.",
        link: "https://wholesale-bowler-01e.notion.site/Cloudier-and-The-Story-Beneath-Their-Songs-Part-1-2273fbb9906a80d2b208e2769563fc6f"
    },
    {
        date: "March 19, 2025",
        tag: "Programming",
        title: "Using Laragon + Laravel to Create a Simple Web App (Vietnamese)",
        summary: "Laragon is a powerful local development environment that makes it easy to create and manage web applications. In this post, we'll walk through the steps to set up a simple web app using Laragon, from installation to deployment.",
        link: "https://candle-millennium-dfe.notion.site/S-d-ng-Laragon-t-o-m-t-ng-d-ng-web-n-gi-n-1bb8b8f4403a808baf06d88cc5df77b4"
    },
    {
        date: "November 20, 2024",
        tag: "Math",
        title: "Huffman Coding (Vietnamese)",
        summary: "Huffman coding is a widely used compression algorithm that reduces the size of data files. By assigning variable-length codes to input characters based on their frequencies, Huffman coding achieves efficient data representation. In this post, we explore the principles behind Huffman coding and its applications.",
        link: "https://candle-millennium-dfe.notion.site/M-Huffman-1448b8f4403a80bcb510ce8769f9df80"
    },

    {
        date: "May 25, 2024",
        tag: "Physics",
        title: "On General Relativity (Vietnamese)",
        summary: "General relativity is a fundamental theory in physics that describes the gravitational force as a curvature of spacetime caused by mass. This post delves into the key concepts of general relativity and its implications for our understanding of the universe.",
        link: "https://candle-millennium-dfe.notion.site/General-Relativity-Thuy-t-T-ng-i-R-ng-b4bcb0163ccb4fe99a70fab10715f3c4"
    },

    {
        date: "April 02, 2024",
        tag: "Gaming, Tech, NES",
        title: "How Zelda Revolutionized Game Saving",
        summary: "The Legend of Zelda was a groundbreaking game that introduced the concept of saving progress in video games. Before Zelda, players had to start over every time they played. This post explores how Zelda's save system changed the gaming landscape forever, allowing players to explore vast worlds without losing their progress.",
        link: "https://candle-millennium-dfe.notion.site/How-Zelda-Revolutionized-Game-Saving-f7200e752c0441dba8cf3c6c14c63599"
    }

];

// Helper functions for blog data management
const BlogData = {
    // Get all posts
    getAllPosts: () => blogPosts,

    // Get posts by tag
    getPostsByTag: (tag) => {
        return blogPosts.filter(post =>
            post.tag.toLowerCase().includes(tag.toLowerCase())
        );
    },

    // Get posts by year
    getPostsByYear: (year) => {
        return blogPosts.filter(post =>
            post.date.includes(year.toString())
        );
    },

    // Get recent posts (limit)
    getRecentPosts: (limit = 5) => {
        return blogPosts.slice(0, limit);
    },

    // Search posts by title or summary
    searchPosts: (query) => {
        const searchTerm = query.toLowerCase();
        return blogPosts.filter(post =>
            post.title.toLowerCase().includes(searchTerm) ||
            post.summary.toLowerCase().includes(searchTerm)
        );
    },

    // Get total count
    getPostCount: () => blogPosts.length,

    // Get unique tags
    getAllTags: () => {
        const tags = new Set();
        blogPosts.forEach(post => {
            post.tag.split(',').forEach(tag => {
                tags.add(tag.trim());
            });
        });
        return Array.from(tags);
    },

    // Add new post (for future functionality)
    addPost: (newPost) => {
        blogPosts.unshift(newPost); // Add to beginning
        return blogPosts;
    }
};

// Export for use in other files
if (typeof module !== 'undefined' && module.exports) {
    module.exports = { blogPosts, BlogData };
}