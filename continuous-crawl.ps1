$hubs = @(
    # Ancient mega-hubs (super connectors)
    "Ancient Greece", "Ancient Rome", "Ancient China", "Ancient India",

    # Core universal concepts
    "Language", "Music", "Religion", "Science", "Food", "Medicine",
    "Education", "Communication", "Transportation", "Energy",

    # Geographic features (universal)
    "Mountain", "River", "Lake", "Island", "Forest", "Ocean",
    "Desert", "Rainforest", "Tundra", "Savanna", "Wetland",

    # Human knowledge domains
    "Anthropology", "Archaeology", "Botany", "Zoology", "Geology",
    "Linguistics", "Philosophy", "Economics", "Literature", "Poetry",

    # Technology & innovation
    "Computer", "Software", "Programming", "Internet", "Satellite",
    "3D printing", "Robotics", "Nanotechnology", "Biotechnology",

    # Cultural expressions
    "Sculpture", "Painting", "Photography", "Cinema", "Theatre",
    "Craft", "Artisan", "Handicraft", "Ceramics", "Woodworking",

    # Social structures
    "Revolution", "Reform", "Activism", "Democracy", "Governance",
    "Law", "Justice", "Rights", "Freedom", "Equality",

    # Material world
    "Metal", "Ore", "Mining", "Forging", "Alloy",
    "Textile", "Fabric", "Clothing", "Fashion",
    "Bronze Age", "Iron Age", "Steel",

    # Maritime & water
    "Ship", "Boat", "Sailing", "Navigation", "Harbor",
    "Fishing", "Maritime transport", "Lighthouse",
    "Irrigation", "Aqueduct", "Dam", "Canal",

    # Trade & commerce
    "Trade", "Commerce", "Market", "Merchant", "Port",
    "Banking", "Finance", "Industry", "Manufacturing",
    "Spice trade", "Trade route", "Caravan",

    # Sports & games
    "Sport", "Game", "Football", "Cricket", "Baseball",
    "Chess", "Martial arts", "Gymnastics", "Competition",

    # Health & medicine
    "Disease", "Health", "Therapy", "Diagnosis", "Surgery",
    "Vaccine", "Antibiotic", "Nutrition", "Anatomy",

    # Religious & philosophical texts
    "Bible", "Quran", "Torah", "Vedas", "Talmud",
    "Logic", "Ethics", "Metaphysics", "Epistemology",

    # Infrastructure
    "Road", "Railway", "Bridge", "Tunnel", "Airport",
    "City", "Town", "Village", "Settlement", "Urban planning",

    # Energy sources
    "Electricity", "Coal", "Oil", "Solar energy", "Nuclear power",
    "Wind power", "Hydropower", "Battery", "Generator",

    # Media & information
    "Newspaper", "Radio", "Television", "Publishing", "Journalism",
    "Book", "Writing", "Printing", "Social media",

    # Regional diversity (carefully selected)
    "South Africa", "Nigeria", "Ethiopia", "Kenya", "Ghana",
    "Mexico", "Peru", "Brazil", "Argentina", "Chile",
    "Indonesia", "Vietnam", "Thailand", "Malaysia", "Philippines",
    "Turkey", "Iran", "Egypt", "Morocco", "Pakistan",

    # Indigenous & traditional
    "Indigenous peoples", "Aboriginal Australians", "Native American religion",
    "Traditional medicine", "Folklore", "Oral tradition", "Cultural heritage",

    # Musical diversity
    "Musical instrument", "Gamelan", "Flamenco", "Sitar", "Tabla",
    "Song", "Melody", "Rhythm", "Jazz", "Opera",

    # Food traditions
    "Cooking", "Cuisine", "Recipe", "Restaurant", "Fermentation",
    "Bread", "Rice", "Spice", "Tea", "Coffee", "Sushi",

    # Historical periods & empires
    "Byzantine Empire", "Ottoman Empire", "Mongol Empire",
    "Inca Empire", "Maya civilization", "Silk Road",

    # Specialized sciences (high-value)
    "Neuroscience", "Ecology", "Paleontology", "Archaeology",
    "Meteorology", "Seismology", "Volcanology", "Oceanography"
)

foreach ($hub in $hubs) {
    Write-Host "Starting crawl from: $hub" -ForegroundColor Cyan
    
    $body = @{
        title = $hub
        depth = 50
        max_pages = 500000
    } | ConvertTo-Json
    
    try {
        $response = Invoke-WebRequest -Uri http://localhost:8080/api/v1/crawl -Method POST -ContentType "application/json" -Body $body
        $result = $response.Content | ConvertFrom-Json
        Write-Host "Job started: $($result.job_id)" -ForegroundColor Green
    } catch {
        Write-Host "Failed to start crawl from $hub" -ForegroundColor Red
    }
    
    Start-Sleep -Seconds 30  # Wait 30s between jobs
}

Write-Host "`nAll crawl jobs started! Monitor with health endpoint:" -ForegroundColor Green
Write-Host "while (`$true) { `$h = (Invoke-WebRequest http://localhost:8080/health).Content | ConvertFrom-Json; Write-Host `"Nodes: `$(`$h.graph.nodes)`"; Start-Sleep 10 }"
